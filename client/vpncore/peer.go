package vpncore

import (
	"encoding/binary"
	"errors"
	"hash/fnv"
	"net"
	"sync"
	"time"
)

// Message types for internal peer protocol.
// These use byte values that are impossible for IP version headers
// (IPv4 = 0x40-0x4F, IPv6 = 0x60-0x6F), so they don't conflict with real packets.
const (
	msgTypePing     = 0x00
	msgTypePong     = 0x01
	msgTypeChatRoom = 0x02 // room broadcast
	msgTypeChatDM   = 0x03 // direct message
)

type Peer struct {
	ID         string    `json:"id"`
	Nickname   string    `json:"nickname"`
	VirtualIP  string    `json:"virtual_ip"`
	LocalAddr  string    `json:"-"`
	RemoteAddr string    `json:"-"`
	PublicKey  string    `json:"-"`
	Connected  bool      `json:"connected"`
	Ping       int       `json:"ping"`
	Path       string    `json:"path"` // p2p | relay | ws | "" — UI / last RTT sample path
	LastSeen   time.Time `json:"-"`
	remoteUDP  *net.UDPAddr
	candidates []*net.UDPAddr // public + local hole-punch targets
	// p2pConfirmed: our ping landed on peer via p2p (echo). Safe for exclusive direct sends.
	p2pConfirmed bool
	cipher       *Cipher
	mu           sync.Mutex
}

type PeerManager struct {
	peers      map[string]*Peer
	mu         sync.RWMutex
	sharedConn *net.UDPConn
	stop       chan struct{}
	wg         sync.WaitGroup
	relay      *RelayClient
	relayAddr  string
	localID    string
	p2pOnly    bool // deprecated: use forceRelay
	forceRelay bool
	// Drop duplicate datagrams (same ciphertext via p2p+relay) within a short window.
	dedupMu sync.Mutex
	dedup   map[uint64]time.Time
	OnPacket    func(fromID string, data []byte)
	OnStatus    func(id string, connected bool)
	OnPing      func(id string, ping int)
	OnChat      func(fromID, nickname, message string, isDM bool)
	SendWSRelay func(toID string, data []byte) error
}

func NewPeerManager() *PeerManager {
	return &PeerManager{
		peers: make(map[string]*Peer),
		stop:  make(chan struct{}),
		dedup: make(map[uint64]time.Time),
	}
}

func (pm *PeerManager) SetLocalID(id string) {
	pm.mu.Lock()
	defer pm.mu.Unlock()
	pm.localID = id
}

func (pm *PeerManager) GetLocalID() string {
	pm.mu.RLock()
	defer pm.mu.RUnlock()
	return pm.localID
}

// SetForceRelay: true = send only via server relay (skip P2P). false = prefer direct.
func (pm *PeerManager) SetForceRelay(enabled bool) {
	pm.mu.Lock()
	defer pm.mu.Unlock()
	pm.forceRelay = enabled
	pm.p2pOnly = false
}

func (pm *PeerManager) Start(conn *net.UDPConn) {
	// Stop any previous goroutines to prevent leak
	pm.Stop()
	pm.mu.Lock()
	pm.sharedConn = conn
	pm.stop = make(chan struct{})
	pm.mu.Unlock()
	pm.wg.Add(2)
	go pm.reader()
	go pm.pingLoop()
}

func (pm *PeerManager) Stop() {
	pm.mu.Lock()
	select {
	case <-pm.stop:
		// already closed
		pm.mu.Unlock()
		return
	default:
		close(pm.stop)
		pm.mu.Unlock()
	}
	pm.wg.Wait()
}

func (pm *PeerManager) AddPeer(id, nickname string) *Peer {
	pm.mu.Lock()
	defer pm.mu.Unlock()
	if existing, ok := pm.peers[id]; ok {
		existing.mu.Lock()
		existing.Nickname = nickname
		existing.mu.Unlock()
		return existing
	}
	p := &Peer{ID: id, Nickname: nickname, LastSeen: time.Now(), Ping: -1}
	pm.peers[id] = p
	return p
}

func (p *Peer) Snapshot() (id, nickname, vip string, connected bool, ping int, path string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.ID, p.Nickname, p.VirtualIP, p.Connected, p.Ping, p.Path
}

func (pm *PeerManager) RemovePeer(id string) {
	pm.mu.Lock()
	defer pm.mu.Unlock()
	if p, ok := pm.peers[id]; ok {
		p.mu.Lock()
		p.Connected = false
		p.remoteUDP = nil
		p.candidates = nil
		p.p2pConfirmed = false
		p.cipher = nil
		p.mu.Unlock()
		delete(pm.peers, id)
	}
	if pm.OnStatus != nil {
		pm.OnStatus(id, false)
	}
}

func (pm *PeerManager) Clear() {
	pm.mu.Lock()
	notifications := make([]string, 0, len(pm.peers))
	for id, p := range pm.peers {
		p.mu.Lock()
		p.Connected = false
		p.remoteUDP = nil
		p.candidates = nil
		p.p2pConfirmed = false
		p.cipher = nil
		p.mu.Unlock()
		notifications = append(notifications, id)
		delete(pm.peers, id)
	}
	pm.mu.Unlock()

	for _, id := range notifications {
		if pm.OnStatus != nil {
			pm.OnStatus(id, false)
		}
	}
}

func (pm *PeerManager) GetPeer(id string) *Peer {
	pm.mu.RLock()
	defer pm.mu.RUnlock()
	return pm.peers[id]
}

func (pm *PeerManager) GetPeerByIP(virtualIP string) *Peer {
	pm.mu.RLock()
	defer pm.mu.RUnlock()
	for _, p := range pm.peers {
		if p.VirtualIP == virtualIP {
			return p
		}
	}
	return nil
}

func (pm *PeerManager) GetPeers() []*Peer {
	pm.mu.RLock()
	defer pm.mu.RUnlock()
	peers := make([]*Peer, 0, len(pm.peers))
	for _, p := range pm.peers {
		peers = append(peers, p)
	}
	return peers
}

func (pm *PeerManager) ConnectToPeer(id string, addr *net.UDPAddr, cipher *Cipher) error {
	if addr == nil {
		return nil
	}
	return pm.ConnectToPeerAddrs(id, []*net.UDPAddr{addr}, cipher)
}

// ConnectToPeerAddrs stores hole-punch candidates (public then local). Path stays
// unproven until a pong arrives via p2p.
func (pm *PeerManager) ConnectToPeerAddrs(id string, addrs []*net.UDPAddr, cipher *Cipher) error {
	p := pm.GetPeer(id)
	if p == nil {
		return nil
	}
	uniq := make([]*net.UDPAddr, 0, len(addrs))
	seen := map[string]bool{}
	for _, a := range addrs {
		if a == nil || a.Port == 0 {
			continue
		}
		k := a.String()
		if seen[k] {
			continue
		}
		seen[k] = true
		uniq = append(uniq, a)
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	if len(uniq) > 0 {
		p.candidates = uniq
		p.remoteUDP = uniq[0]
		if p.Path == "" || p.Path == "relay" || p.Path == "ws" {
			// aspirational until pong; UI may still show relay until proven
		}
	}
	if cipher != nil {
		p.cipher = cipher
	}
	p.Connected = true
	p.LastSeen = time.Now()
	if pm.OnStatus != nil {
		pm.OnStatus(id, true)
	}
	return nil
}

// SetPeerCipher installs session crypto even when direct UDP is unavailable (relay/WS only).
func (pm *PeerManager) SetPeerCipher(id string, cipher *Cipher) {
	p := pm.GetPeer(id)
	if p == nil || cipher == nil {
		return
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	p.cipher = cipher
	p.Connected = true
	p.LastSeen = time.Now()
	if p.Path == "" {
		p.Path = "relay"
	}
	if pm.OnStatus != nil {
		pm.OnStatus(id, true)
	}
}

func (pm *PeerManager) reader() {
	defer pm.wg.Done()
	buf := make([]byte, 65535)
	for {
		select {
		case <-pm.stop:
			return
		default:
		}

		// Capture shared state under RLock
		pm.mu.RLock()
		conn := pm.sharedConn
		relayAddr := pm.relayAddr
		pm.mu.RUnlock()

		if conn == nil {
			return
		}

		n, remote, err := conn.ReadFromUDP(buf)
		if err != nil {
			return
		}
		if n < 2 {
			continue
		}
		remoteStr := remote.String()

		// Check if this is a relayed packet
		isRelay := relayAddr != "" && remoteStr == relayAddr

		var srcVIP string
		var payload []byte

		if isRelay {
			// Relay packet format: [src_vip_len:1][src_vip][encrypted_payload]
			if n < 2 {
				continue
			}
			srcVipLen := int(buf[0])
			if srcVipLen < 1 || 1+srcVipLen >= n {
				continue
			}
			srcVIP = string(buf[1 : 1+srcVipLen])
			payload = buf[1+srcVipLen : n]
		} else {
			// Direct P2P: find peer by source address
			p := pm.findPeerByAddr(remoteStr)
			if p == nil {
				// Just continue silently — don't send entangled-pong
				continue
			}
			srcVIP = p.VirtualIP
			payload = buf[:n]
		}

		pm.processPeerPayload(srcVIP, payload, isRelay)
	}
}

func (pm *PeerManager) HandleRelayPacket(data []byte) {
	if len(data) < 2 {
		return
	}
	srcVipLen := int(data[0])
	if srcVipLen < 1 || 1+srcVipLen >= len(data) {
		return
	}
	srcVIP := string(data[1 : 1+srcVipLen])
	pm.processPeerPayload(srcVIP, data[1+srcVipLen:], true)
}

func (pm *PeerManager) duplicatePayload(srcVIP string, payload []byte) bool {
	h := fnv.New64a()
	_, _ = h.Write([]byte(srcVIP))
	_, _ = h.Write(payload)
	key := h.Sum64()
	now := time.Now()
	pm.dedupMu.Lock()
	defer pm.dedupMu.Unlock()
	if pm.dedup == nil {
		pm.dedup = make(map[uint64]time.Time)
	}
	if t, ok := pm.dedup[key]; ok && now.Sub(t) < 2*time.Second {
		return true
	}
	pm.dedup[key] = now
	if len(pm.dedup) > 256 {
		for k, t := range pm.dedup {
			if now.Sub(t) > 2*time.Second {
				delete(pm.dedup, k)
			}
		}
	}
	return false
}

func (pm *PeerManager) processPeerPayload(srcVIP string, payload []byte, isRelay bool) {
	p := pm.GetPeerByIP(srcVIP)
	if p == nil {
		Logger.Printf("processPeerPayload: peer not found for srcVIP=%s", srcVIP)
		return
	}
	if len(payload) < 2 {
		Logger.Printf("processPeerPayload: payload too short (%d bytes) from %s (%s)", len(payload), p.ID, p.Nickname)
		return
	}
	pktLen := int(binary.BigEndian.Uint16(payload[:2]))
	if pktLen+2 > len(payload) {
		Logger.Printf("processPeerPayload: pktLen %d exceeds payload %d from %s (%s)", pktLen, len(payload), p.ID, p.Nickname)
		return
	}
	// Same encrypted datagram often arrives on both p2p and relay while punching.
	if pm.duplicatePayload(srcVIP, payload[:2+pktLen]) {
		return
	}
	p.mu.Lock()
	cipher := p.cipher
	p.mu.Unlock()
	if cipher == nil {
		Logger.Printf("processPeerPayload: cipher is nil for %s (%s)", p.ID, p.Nickname)
		return
	}
	ct := payload[2 : 2+pktLen]
	plaintext, err := cipher.Decrypt(ct)
	if err != nil {
		Logger.Printf("processPeerPayload: decrypt failed for %s (%s): %v", p.ID, p.Nickname, err)
		return
	}

	if len(plaintext) == 0 {
		return
	}

	p.mu.Lock()
	p.LastSeen = time.Now()

	// Internal protocol messages (ping/pong) use bytes that are invalid IP version headers
	switch plaintext[0] {
	case msgTypePing:
		if len(plaintext) >= 9 {
			source := "relay"
			if !isRelay {
				source = "p2p"
			}
			id := p.ID
			nickname := p.Nickname
			ts := make([]byte, 8)
			copy(ts, plaintext[1:9])
			p.mu.Unlock()
			Logger.Printf("ping received [%s] from %s (%s)", source, id, nickname)
			pm.sendPong(p, ts, isRelay)
		} else {
			p.mu.Unlock()
		}
		return

	case msgTypePong:
		if len(plaintext) >= 9 {
			ts := int64(binary.BigEndian.Uint64(plaintext[1:9]))
			rtt := time.Now().UnixNano() - ts
			if rtt < 0 {
				rtt = 0
			}
			pingMs := int(rtt / 1e6)
			source := "relay"
			if !isRelay {
				source = "p2p"
			}
			// byte9 = how the peer received our ping: 0=p2p, 1=relay (1.2.6+)
			pingVia := byte(0xff)
			if len(plaintext) >= 10 {
				pingVia = plaintext[9]
			}
			id := p.ID
			nickname := p.Nickname
			p.Ping = pingMs
			if pingVia == 0 {
				p.p2pConfirmed = true
			}
			// UI path follows the RTT sample: a p2p-delivered pong (~30ms) is Direct
			// even when ping_via=relay (mixed path). Exclusive sends still need p2pConfirmed.
			switch {
			case !isRelay || pingVia == 0:
				p.Path = "p2p"
			case p.Path != "p2p":
				if source == "ws" {
					p.Path = "ws"
				} else {
					p.Path = "relay"
				}
			}
			p.mu.Unlock()
			via := "unknown"
			switch pingVia {
			case 0:
				via = "p2p"
			case 1:
				via = "relay"
			}
			Logger.Printf("pong received [%s] from %s (%s): %dms (ping_via=%s)", source, id, nickname, pingMs, via)
			if pm.OnPing != nil {
				pm.OnPing(id, pingMs)
			}
		} else {
			p.mu.Unlock()
		}
		return

	case msgTypeChatRoom, msgTypeChatDM:
		if len(plaintext) < 2 {
			p.mu.Unlock()
			return
		}
		isDM := plaintext[0] == msgTypeChatDM
		nickname := p.Nickname
		msg := string(plaintext[1:])
		runes := []rune(msg)
		if len(runes) > 4096 {
			msg = string(runes[:4096])
		}
		fromID := p.ID
		p.mu.Unlock()
		if pm.OnChat != nil {
			pm.OnChat(fromID, nickname, msg, isDM)
		}
		return
	}

	p.mu.Unlock()

	if pm.OnPacket != nil {
		pm.OnPacket(p.ID, plaintext)
	}
}

func (pm *PeerManager) findPeerByAddr(addr string) *Peer {
	pm.mu.RLock()
	defer pm.mu.RUnlock()
	host := addr
	if h, _, err := net.SplitHostPort(addr); err == nil {
		host = h
	}
	for _, peer := range pm.peers {
		peer.mu.Lock()
		var match bool
		if peer.remoteUDP != nil {
			match = peer.remoteUDP.String() == addr || peer.remoteUDP.IP.String() == host
			if match && peer.remoteUDP.String() != addr {
				if a, err := net.ResolveUDPAddr("udp", addr); err == nil {
					peer.remoteUDP = a
					// Keep learned mapping in the candidate set for later sends.
					peer.candidates = prependCandidate(peer.candidates, a)
				}
			}
		}
		if !match {
			for _, c := range peer.candidates {
				if c == nil {
					continue
				}
				if c.String() == addr || c.IP.String() == host {
					match = true
					if a, err := net.ResolveUDPAddr("udp", addr); err == nil {
						peer.remoteUDP = a
						peer.candidates = prependCandidate(peer.candidates, a)
					}
					break
				}
			}
		}
		peer.mu.Unlock()
		if match {
			return peer
		}
	}
	return nil
}

func prependCandidate(cands []*net.UDPAddr, a *net.UDPAddr) []*net.UDPAddr {
	if a == nil {
		return cands
	}
	out := make([]*net.UDPAddr, 0, 1+len(cands))
	out = append(out, a)
	k := a.String()
	for _, c := range cands {
		if c == nil || c.String() == k {
			continue
		}
		out = append(out, c)
	}
	return out
}

func (pm *PeerManager) SetRelayClient(r *RelayClient) {
	pm.mu.Lock()
	defer pm.mu.Unlock()
	pm.relay = r
	if r != nil {
		addr := r.RelayAddr()
		if addr != nil {
			pm.relayAddr = addr.String()
		}
	}
}

func (pm *PeerManager) sendPlaintext(p *Peer, plaintext []byte) error {
	// Capture PeerManager shared state under read lock before locking p.mu
	pm.mu.RLock()
	forceRelay := pm.forceRelay
	sharedConn := pm.sharedConn
	sendWSRelay := pm.SendWSRelay
	relay := pm.relay
	pm.mu.RUnlock()

	p.mu.Lock()

	if p.cipher == nil {
		p.mu.Unlock()
		return errors.New("peer cipher not ready")
	}

	encrypted, err := p.cipher.Encrypt(plaintext)
	if err != nil {
		p.mu.Unlock()
		return err
	}
	remoteUDP := p.remoteUDP
	cands := append([]*net.UDPAddr(nil), p.candidates...)
	vip := p.VirtualIP
	id := p.ID
	neverPonged := p.Ping < 0
	path := p.Path
	p2pConfirmed := p.p2pConfirmed
	p.mu.Unlock()

	pkt := make([]byte, 2+len(encrypted))
	binary.BigEndian.PutUint16(pkt[:2], uint16(len(encrypted)))
	copy(pkt[2:], encrypted)

	probe := len(plaintext) > 0 && (plaintext[0] == msgTypePing || plaintext[0] == msgTypePong)
	// Exclusive P2P only when peer echoed that our ping arrived via p2p — not merely
	// when Path shows Direct from a mixed-path RTT sample.
	provenP2P := !neverPonged && p2pConfirmed && !forceRelay

	writeP2P := func(allCandidates bool) bool {
		if sharedConn == nil {
			return false
		}
		targets := make([]*net.UDPAddr, 0, 1+len(cands))
		seen := map[string]bool{}
		add := func(a *net.UDPAddr) {
			if a == nil || a.Port == 0 {
				return
			}
			k := a.String()
			if seen[k] {
				return
			}
			seen[k] = true
			targets = append(targets, a)
		}
		add(remoteUDP)
		if allCandidates {
			for _, a := range cands {
				add(a)
			}
		}
		ok := false
		for _, addr := range targets {
			if _, err := sharedConn.WriteToUDP(pkt, addr); err == nil {
				ok = true
			}
		}
		return ok
	}

	if !forceRelay {
		if provenP2P {
			if writeP2P(false) {
				pm.setPeerPath(p, "p2p")
				return nil
			}
			// Direct write failed — fall through to relay.
		} else if probe {
			// Hole-punch probes spray candidates; relay keeps session alive until proven.
			_ = writeP2P(true)
		}
	}
	// App data while unproven / force-relay: relay/WS only (no dual-send → no doubled chat).

	relayOK := false
	if vip != "" && relay != nil {
		if err := relay.SendToPeer(vip, pkt); err == nil {
			relayOK = true
		}
	}

	// WS when UDP relay fails, or until first pong (helps before relay REG completes).
	wsOK := false
	if sendWSRelay != nil && (!relayOK || neverPonged) {
		if err := sendWSRelay(id, pkt); err == nil {
			wsOK = true
		} else if !relayOK {
			return err
		}
	}

	if relayOK {
		if forceRelay || (!probe && path != "p2p") {
			pm.setPeerPath(p, "relay")
		}
		return nil
	}
	if wsOK {
		if forceRelay || (!probe && path != "p2p") {
			pm.setPeerPath(p, "ws")
		}
		return nil
	}
	if probe && !forceRelay {
		// Optimistic hole-punch with no relay yet.
		return nil
	}
	return errors.New("no transport path available")
}

func (pm *PeerManager) setPeerPath(p *Peer, path string) {
	p.mu.Lock()
	p.Path = path
	p.mu.Unlock()
}

func (pm *PeerManager) SendToPeer(id string, data []byte) error {
	p := pm.GetPeer(id)
	if p == nil {
		return nil
	}
	return pm.sendPlaintext(p, data)
}

// PingPeer sends an immediate latency probe to one peer. Resets displayed ping
// until the pong arrives (OnPing). Returns an error if the peer is missing or offline.
func (pm *PeerManager) PingPeer(id string) error {
	p := pm.GetPeer(id)
	if p == nil {
		return errors.New("peer not found")
	}
	p.mu.Lock()
	connected := p.Connected
	if connected {
		p.Ping = -1
	}
	p.mu.Unlock()
	if !connected {
		return errors.New("peer offline")
	}
	pm.sendPing(p)
	return nil
}

func (pm *PeerManager) sendPing(p *Peer) {
	p.mu.Lock()
	connected := p.Connected
	p.mu.Unlock()

	if !connected {
		return
	}

	now := time.Now().UnixNano()
	data := make([]byte, 9)
	data[0] = msgTypePing
	binary.BigEndian.PutUint64(data[1:], uint64(now))

	Logger.Printf("ping sending to %s (%s)", p.ID, p.Nickname)
	pm.sendPlaintext(p, data)
}

func (pm *PeerManager) sendPong(p *Peer, ts []byte, pingViaRelay bool) {
	if len(ts) != 8 {
		return
	}
	data := make([]byte, 10)
	data[0] = msgTypePong
	copy(data[1:9], ts)
	if pingViaRelay {
		data[9] = 1
	} else {
		data[9] = 0
	}

	pm.sendPlaintext(p, data)
}

func (pm *PeerManager) pingLoop() {
	defer pm.wg.Done()
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-pm.stop:
			return
		case <-ticker.C:
			for _, p := range pm.GetPeers() {
				p.mu.Lock()
				connected := p.Connected
				p.mu.Unlock()
				if connected {
					pm.sendPing(p)
				}
			}
		}
	}
}

func (pm *PeerManager) BroadcastToAll(data []byte, excludeID string) {
	for _, p := range pm.GetPeers() {
		if p.ID != excludeID {
			pm.SendToPeer(p.ID, data)
		}
	}
}

func (pm *PeerManager) SendChat(toID, message string) error {
	// Rune-safe truncation
	runes := []rune(message)
	if len(runes) > 1000 {
		message = string(runes[:1000])
	}
	data := make([]byte, 1+len(message))
	data[0] = msgTypeChatDM
	copy(data[1:], message)
	return pm.SendToPeer(toID, data)
}

func (pm *PeerManager) BroadcastChat(message string, excludeID string) error {
	// Rune-safe truncation
	runes := []rune(message)
	if len(runes) > 1000 {
		message = string(runes[:1000])
	}
	data := make([]byte, 1+len(message))
	data[0] = msgTypeChatRoom
	copy(data[1:], message)
	var firstErr error
	for _, p := range pm.GetPeers() {
		if p.ID == excludeID {
			continue
		}
		if err := pm.SendToPeer(p.ID, data); err != nil && firstErr == nil {
			firstErr = err
		}
	}
	return firstErr
}

func (pm *PeerManager) UpdatePeerInfo(id, virtualIP, localAddr, publicKey string) {
	pm.mu.Lock()
	p, exists := pm.peers[id]
	if !exists {
		p = &Peer{ID: id, LastSeen: time.Now(), Ping: -1}
		pm.peers[id] = p
	}
	p.mu.Lock()
	pm.mu.Unlock()
	p.VirtualIP = virtualIP
	p.LocalAddr = localAddr
	p.PublicKey = publicKey
	p.mu.Unlock()
}
