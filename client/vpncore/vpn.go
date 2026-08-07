package vpncore

import (
	"fmt"
	"net"
	"sync"
	"time"
)

func getLocalIPv4() string {
	dnsServers := []string{"8.8.8.8:53", "1.1.1.1:53", "8.8.4.4:53"}
	for _, dns := range dnsServers {
		conn, err := net.Dial("udp4", dns)
		if err != nil {
			continue
		}
		localAddr := conn.LocalAddr().(*net.UDPAddr)
		conn.Close()
		return localAddr.IP.String()
	}
	return "127.0.0.1"
}

type VPNCore struct {
	config         *VPNConfig
	signaling      *SignalingClient
	peers          *PeerManager
	router         *PacketRouter
	tun            *TUNAdapter
	keyPair        *KeyPair
	relay          *RelayClient
	p2pOnly        bool
	status         ConnectionStatus
	mu             sync.Mutex
	listenerConn   *net.UDPConn
	listenerMu     sync.Mutex
	stopping       bool
	reconnectGen   int
	authOK         chan struct{}
	lastRoomName   string
	lastRoomPass   string
	lastIsOwner    bool
	relayToken     string
	OnLog          func(string)
	OnStatusChange func(ConnectionStatus)
	OnPeersChange  func([]*Peer)
	OnError        func(string)
	OnChat         func(fromID, nickname, message string, isDM bool)
	OnSystemChat   func(text string)
	OnRoomDeleted  func(name string)
}

func (v *VPNCore) log(format string, args ...interface{}) {
	msg := fmt.Sprintf(format, args...)
	if v.OnLog != nil {
		v.OnLog(msg)
	}
}

type VPNConfig struct {
	ServerAddr string
	Nickname   string
	P2POnly    bool
	STUNServer string
	MTU        int
	DNSServer  string
	SOCKS5Addr string
	AuthToken  string
}

type ConnectionStatus struct {
	Connected    bool   `json:"connected"`
	Reconnecting bool   `json:"reconnecting"`
	Server       string `json:"server"`
	Room         string `json:"room"`
	VirtualIP    string `json:"virtual_ip"`
	PeerCount    int    `json:"peer_count"`
	IsOwner      bool   `json:"is_owner"`
	Phase        string `json:"phase"` // idle|dialing|auth|ready|error
}

func NewVPNCore(config *VPNConfig) *VPNCore {
	v := &VPNCore{
		config:  config,
		peers:   NewPeerManager(),
		p2pOnly: config.P2POnly,
		status:  ConnectionStatus{Phase: "idle"},
	}
	v.peers.SetP2POnly(config.P2POnly)
	v.peers.OnPing = func(id string, ping int) {
		v.updatePeers()
	}
	return v
}

func (v *VPNCore) SetP2POnly(enabled bool) {
	v.mu.Lock()
	defer v.mu.Unlock()
	v.p2pOnly = enabled
	v.peers.SetP2POnly(enabled)
}

// ApplyTUNSettings updates MTU/DNS on a live TUN (no-op if TUN not created yet).
func (v *VPNCore) ApplyTUNSettings(mtu int, dnsServer string) {
	v.mu.Lock()
	if v.config != nil {
		v.config.MTU = mtu
		v.config.DNSServer = dnsServer
	}
	tun := v.tun
	v.mu.Unlock()
	if tun == nil {
		return
	}
	if mtu > 0 {
		tun.SetMTU(mtu)
	}
	if dnsServer != "" {
		tun.SetDNS(dnsServer)
	}
}

func (v *VPNCore) Start() error {
	v.mu.Lock()
	v.stopping = false
	v.status.Phase = "dialing"
	v.mu.Unlock()
	v.updateStatus()

	kp, err := GenerateKeyPair()
	if err != nil {
		return fmt.Errorf("key generation failed: %v", err)
	}
	v.keyPair = kp
	v.log("Key pair generated")

	if err := v.connectSignaling(); err != nil {
		v.mu.Lock()
		v.status.Phase = "error"
		v.mu.Unlock()
		v.updateStatus()
		return err
	}

	v.mu.Lock()
	v.status.Connected = true
	v.status.Reconnecting = false
	v.status.Server = v.config.ServerAddr
	v.status.Phase = "ready"
	v.mu.Unlock()
	v.updateStatus()
	return nil
}

func (v *VPNCore) connectSignaling() error {
	signaling := NewSignalingClient()
	signaling.SetLogger(v.log)
	if v.config.SOCKS5Addr != "" {
		signaling.SetSOCKS5(v.config.SOCKS5Addr)
	}
	signaling.SetAuthToken(v.config.AuthToken)

	authOK := make(chan struct{})
	var authOnce sync.Once
	v.mu.Lock()
	v.authOK = authOK
	v.mu.Unlock()

	signaling.OnLocalID = func(id string) {
		v.peers.SetLocalID(id)
		v.log("Local peer ID: %s", id)
		v.mu.Lock()
		v.status.Phase = "auth"
		v.mu.Unlock()
		v.updateStatus()
		authOnce.Do(func() { close(authOK) })
	}
	v.wireSignalingHandlers(signaling)
	v.signaling = signaling
	v.log("Signaling client created")

	if v.tun == nil {
		v.tun = NewTUNAdapter()
		v.tun.OnLog = v.log
	}
	if v.config.MTU > 0 {
		v.tun.SetMTU(v.config.MTU)
	}
	if v.config.DNSServer != "" {
		v.tun.SetDNS(v.config.DNSServer)
	}

	if err := signaling.Connect(v.config.ServerAddr, v.config.Nickname); err != nil {
		return err
	}
	return nil
}

func (v *VPNCore) wireSignalingHandlers(signaling *SignalingClient) {
	signaling.OnRoomJoined = func(room string, isOwner bool, virtualIP string, peers []map[string]string, relayToken string) {
		v.log("OnRoomJoined: room=%s owner=%v peers=%d", room, isOwner, len(peers))

		if virtualIP == "" {
			v.log("OnRoomJoined: empty virtual_ip from server — aborting room setup")
			if v.OnError != nil {
				v.OnError("Server assigned no virtual IP")
			}
			return
		}
		v.log("Virtual IP: %s", virtualIP)

		v.mu.Lock()
		v.status.Room = room
		v.status.PeerCount = len(peers)
		v.status.VirtualIP = virtualIP
		v.status.IsOwner = isOwner
		v.lastRoomName = room
		v.lastIsOwner = isOwner
		v.relayToken = relayToken
		v.mu.Unlock()
		v.updateStatus()

		v.mu.Lock()
		if v.relay != nil {
			v.relay.Stop()
		}
		v.relay = NewRelayClient(v.config.ServerAddr)
		v.mu.Unlock()

		v.relay.SetLogger(func(f string, a ...interface{}) { v.log(f, a...) })
		v.relay.SetToken(relayToken)
		if err := v.relay.Start(virtualIP); err != nil {
			v.log("Relay start error: %v", err)
		} else {
			v.log("Relay client started for %s via %s", virtualIP, v.config.ServerAddr)
			v.peers.SetRelayClient(v.relay)
		}

		go v.startPeerListener()

		for _, peer := range peers {
			if peer["id"] != "" {
				v.peers.AddPeer(peer["id"], peer["nickname"])
				v.log("Added existing peer: %s (%s)", peer["nickname"], peer["id"])
			}
		}
		v.updatePeers()

		if err := v.tun.Start(virtualIP); err != nil {
			v.log("TUN start error: %v", err)
			if v.OnError != nil {
				v.OnError("TUN adapter error (run as Administrator?): " + err.Error())
			}
		} else {
			v.log("TUN adapter started")
			v.router = NewPacketRouter(virtualIP, v.peers)
			v.router.OnLog = v.log
			v.tun.OnPacket = func(data []byte) {
				v.router.HandleTUNPacket(data)
			}
			v.peers.OnPacket = func(fromID string, data []byte) {
				v.router.HandlePeerPacket(fromID, data)
			}
			v.peers.OnChat = func(fromID, nickname, message string, isDM bool) {
				if v.OnChat != nil {
					v.OnChat(fromID, nickname, message, isDM)
				}
			}
			v.router.OnSendToTUN = func(data []byte) {
				v.tun.Write(data)
			}
			v.router.OnAddRoute = func(peerIP string) {
				v.tun.AddRoute(peerIP)
			}

			for _, p := range v.peers.GetPeers() {
				p.mu.Lock()
				vip := p.VirtualIP
				p.mu.Unlock()
				if vip != "" {
					v.router.AddRoute(vip)
				}
			}
		}

		v.peers.SendWSRelay = func(toID string, data []byte) error {
			v.mu.Lock()
			vip := v.status.VirtualIP
			sig := v.signaling
			v.mu.Unlock()
			if sig == nil {
				return fmt.Errorf("no signaling")
			}
			pkt := make([]byte, 1+len(vip)+len(data))
			pkt[0] = byte(len(vip))
			copy(pkt[1:], vip)
			copy(pkt[1+len(vip):], data)
			sig.SendRelayData(toID, pkt)
			return nil
		}
		signaling.OnRelayData = func(fromID string, data []byte) {
			v.peers.HandleRelayPacket(data)
		}
	}

	signaling.OnRoomDeleted = func(name string) {
		v.log("Room deleted: %s", name)
		v.cleanupRoomLocal()
		if v.OnRoomDeleted != nil {
			v.OnRoomDeleted(name)
		}
		if v.OnSystemChat != nil {
			v.OnSystemChat("Room deleted: " + name)
		}
	}

	signaling.OnPeerJoined = func(id, nickname string) {
		v.log("OnPeerJoined: id=%s nickname=%s", id, nickname)
		v.peers.AddPeer(id, nickname)
		v.mu.Lock()
		v.status.PeerCount = len(v.peers.GetPeers())
		v.mu.Unlock()
		v.updateStatus()
		v.updatePeers()
		if v.OnSystemChat != nil {
			v.OnSystemChat(nickname + " joined")
		}
	}

	signaling.OnPeerLeft = func(id string) {
		v.log("OnPeerLeft: id=%s", id)
		nick := id
		if p := v.peers.GetPeer(id); p != nil {
			p.mu.Lock()
			if p.Nickname != "" {
				nick = p.Nickname
			}
			p.mu.Unlock()
		}
		v.peers.RemovePeer(id)
		v.mu.Lock()
		v.status.PeerCount = len(v.peers.GetPeers())
		v.mu.Unlock()
		v.updateStatus()
		v.updatePeers()
		if v.OnSystemChat != nil {
			v.OnSystemChat(nick + " left")
		}
	}

	signaling.OnPeerUpdated = func(id, virtualIP, localAddr, publicAddr, publicKey, cryptoMode string) {
		v.log("OnPeerUpdated: id=%s ip=%s local=%s public=%s crypto=%s", id, virtualIP, localAddr, publicAddr, cryptoMode)
		v.peers.UpdatePeerInfo(id, virtualIP, localAddr, publicKey)

		if virtualIP != "" && v.router != nil {
			v.router.AddRoute(virtualIP)
		}

		pubKeyBytes, err := DecodePublicKey(publicKey)
		if err != nil {
			v.log("DecodePublicKey failed for %s: %v", id, err)
			v.updatePeers()
			return
		}

		secret, err := ComputeSharedSecret(v.keyPair.PrivateKey, pubKeyBytes)
		if err != nil {
			v.log("ComputeSharedSecret failed for %s: %v", id, err)
			v.updatePeers()
			return
		}
		if cryptoMode != "" && cryptoMode != CryptoHKDF {
			v.log("Peer %s advertised unsupported crypto %q (require %s)", id, cryptoMode, CryptoHKDF)
		}
		cipher, err := NewCipher(secret)
		if err != nil {
			v.log("Cipher creation failed for %s: %v", id, err)
			v.updatePeers()
			return
		}

		// Always install cipher so chat/data work over relay even if P2P fails.
		v.peers.SetPeerCipher(id, cipher)

		targets := []string{}
		if publicAddr != "" {
			targets = append(targets, publicAddr)
		}
		if localAddr != "" {
			targets = append(targets, localAddr)
		}

		connected := false
		for _, target := range targets {
			addr, err := net.ResolveUDPAddr("udp", target)
			if err != nil || addr.Port == 0 {
				continue
			}
			v.log("P2P connecting to %s at %s", id, target)
			if err := v.peers.ConnectToPeer(id, addr, cipher); err != nil {
				v.log("P2P connect to %s at %s failed: %v", id, target, err)
			} else {
				v.log("P2P connected to %s via %s", id, target)
				connected = true
				break
			}
		}
		if !connected {
			v.log("P2P unavailable for %s — using relay/WS", id)
		}
		v.updatePeers()
	}

	signaling.OnError = func(msg string) {
		v.log("OnError: %s", msg)
		if v.OnError != nil {
			v.OnError(msg)
		}
	}

	signaling.OnDisconnected = func(intentional bool) {
		v.log("OnDisconnected intentional=%v", intentional)
		v.mu.Lock()
		stopping := v.stopping
		room := v.lastRoomName
		pass := v.lastRoomPass
		v.mu.Unlock()

		if intentional || stopping {
			v.cleanupAfterDisconnect()
			return
		}

		v.mu.Lock()
		v.status.Connected = false
		v.status.Reconnecting = true
		v.status.Phase = "dialing"
		v.reconnectGen++
		gen := v.reconnectGen
		v.mu.Unlock()
		v.updateStatus()

		go v.reconnectLoop(gen, room, pass)
	}
}

func (v *VPNCore) reconnectLoop(gen int, room, pass string) {
	delays := []time.Duration{1 * time.Second, 2 * time.Second, 4 * time.Second, 8 * time.Second, 15 * time.Second}
	for i, d := range delays {
		v.mu.Lock()
		if v.stopping || v.reconnectGen != gen {
			v.mu.Unlock()
			return
		}
		v.mu.Unlock()

		v.log("Reconnect attempt %d after %v", i+1, d)
		time.Sleep(d)

		v.mu.Lock()
		if v.stopping || v.reconnectGen != gen {
			v.mu.Unlock()
			return
		}
		v.mu.Unlock()

		v.peers.Clear()
		if err := v.connectSignaling(); err != nil {
			v.log("Reconnect failed: %v", err)
			continue
		}

		v.mu.Lock()
		authOK := v.authOK
		v.mu.Unlock()
		if authOK != nil {
			select {
			case <-authOK:
			case <-time.After(15 * time.Second):
				v.log("Reconnect auth timeout")
				continue
			}
		}

		v.mu.Lock()
		if v.stopping || v.reconnectGen != gen {
			v.mu.Unlock()
			return
		}
		v.status.Connected = true
		v.status.Reconnecting = false
		v.status.Server = v.config.ServerAddr
		v.status.Phase = "ready"
		v.mu.Unlock()
		v.updateStatus()

		if room != "" {
			v.log("Re-joining room %s after reconnect", room)
			v.signaling.JoinRoom(room, pass)
		}
		return
	}

	v.log("Reconnect exhausted")
	v.cleanupAfterDisconnect()
	if v.OnError != nil {
		v.OnError("Disconnected — reconnect failed")
	}
}

func (v *VPNCore) cleanupRoomLocal() {
	if v.tun != nil {
		v.tun.Close()
	}
	v.mu.Lock()
	if v.relay != nil {
		v.relay.Stop()
		v.relay = nil
	}
	v.status.Room = ""
	v.status.PeerCount = 0
	v.status.VirtualIP = ""
	v.status.IsOwner = false
	v.lastRoomName = ""
	v.lastRoomPass = ""
	v.mu.Unlock()
	v.peers.Clear()
	v.updateStatus()
	v.updatePeers()
}

func (v *VPNCore) cleanupAfterDisconnect() {
	v.mu.Lock()
	v.status.Connected = false
	v.status.Reconnecting = false
	v.status.Room = ""
	v.status.PeerCount = 0
	v.status.VirtualIP = ""
	v.status.IsOwner = false
	v.status.Phase = "idle"
	if v.relay != nil {
		v.relay.Stop()
		v.relay = nil
	}
	v.mu.Unlock()
	if v.tun != nil {
		v.tun.Close()
	}
	v.peers.Clear()
	v.updateStatus()
	v.updatePeers()
}

func (v *VPNCore) Stop() {
	v.mu.Lock()
	v.stopping = true
	v.reconnectGen++
	if v.relay != nil {
		v.relay.Stop()
	}
	sig := v.signaling
	if v.listenerConn != nil {
		v.listenerConn.Close()
		v.listenerConn = nil
	}
	v.mu.Unlock()

	if sig != nil {
		sig.Close()
	}
	if v.tun != nil {
		v.tun.Close()
	}
	v.peers.Stop()
	v.peers.Clear()

	v.mu.Lock()
	v.status = ConnectionStatus{Phase: "idle"}
	v.mu.Unlock()
	v.updateStatus()
	v.updatePeers()
}

func (v *VPNCore) CreateRoom(name, password string) {
	v.log("CreateRoom called: %s", name)
	v.mu.Lock()
	v.lastRoomName = name
	v.lastRoomPass = password
	v.mu.Unlock()
	v.peers.Clear()
	v.signaling.CreateRoom(name, password)
}

func (v *VPNCore) JoinRoom(name, password string) {
	v.log("JoinRoom called: %s", name)
	v.mu.Lock()
	v.lastRoomName = name
	v.lastRoomPass = password
	v.mu.Unlock()
	v.peers.Clear()
	v.signaling.JoinRoom(name, password)
}

func (v *VPNCore) LeaveRoom() {
	v.log("LeaveRoom called")
	if v.signaling != nil {
		v.signaling.LeaveRoom()
	}
	v.cleanupRoomLocal()
}

func (v *VPNCore) DeleteRoom(name string) {
	v.log("DeleteRoom called: %s", name)
	if v.signaling != nil {
		v.signaling.DeleteRoom(name)
	}
}

func (v *VPNCore) GetStatus() ConnectionStatus {
	v.mu.Lock()
	defer v.mu.Unlock()
	return v.status
}

func (v *VPNCore) GetPeers() []*Peer {
	return v.peers.GetPeers()
}

func (v *VPNCore) SendChat(toID, message string) error {
	return v.peers.SendChat(toID, message)
}

func (v *VPNCore) BroadcastChat(message string) error {
	return v.peers.BroadcastChat(message, v.peers.GetLocalID())
}

func (v *VPNCore) PingPeer(id string) error {
	if err := v.peers.PingPeer(id); err != nil {
		return err
	}
	v.updatePeers()
	return nil
}

func (v *VPNCore) startPeerListener() {
	v.listenerMu.Lock()
	defer v.listenerMu.Unlock()

	v.peers.Stop()
	v.mu.Lock()
	if v.listenerConn != nil {
		v.listenerConn.Close()
		v.listenerConn = nil
	}
	v.mu.Unlock()

	addr, _ := net.ResolveUDPAddr("udp4", ":0")
	conn, err := net.ListenUDP("udp4", addr)
	if err != nil {
		v.log("peer listener error: %v", err)
		return
	}
	v.mu.Lock()
	v.listenerConn = conn
	v.mu.Unlock()

	localIP := getLocalIPv4()
	localPort := conn.LocalAddr().(*net.UDPAddr).Port
	listenAddr := fmt.Sprintf("%s:%d", localIP, localPort)
	v.log("UDP listener started on %s", listenAddr)

	v.mu.Lock()
	signaling := v.signaling
	vip := v.status.VirtualIP
	pubKey := v.keyPair.PublicKey
	stunServer := v.config.STUNServer
	v.mu.Unlock()

	externalAddr := ""
	if signaling != nil && vip != "" {
		stunAddr := stunServer
		if stunAddr == "" {
			stunAddr = "stun.l.google.com:19302"
		}
		ea, err := DiscoverPublicAddr(stunAddr, 5*time.Second, conn)
		if err == nil {
			externalAddr = ea
			v.log("STUN external address: %s", externalAddr)
		} else {
			v.log("STUN discovery failed: %v (will use server-derived addr)", err)
		}
	}

	v.peers.Start(conn)

	v.mu.Lock()
	if v.relay != nil {
		v.relay.SetConn(conn)
		v.relay.RegisterNow()
		v.log("Relay client bound to shared UDP conn")
	}
	v.mu.Unlock()

	if signaling != nil && vip != "" {
		pubKeyStr := EncodePublicKey(pubKey)
		signaling.SendPeerInfo(vip, listenAddr, pubKeyStr, externalAddr)
		v.log("Peer info sent with addr %s external=%s", listenAddr, externalAddr)
	}
}

func (v *VPNCore) updateStatus() {
	if v.OnStatusChange != nil {
		v.mu.Lock()
		status := v.status
		v.mu.Unlock()
		v.OnStatusChange(status)
	}
}

func (v *VPNCore) updatePeers() {
	if v.OnPeersChange != nil {
		v.OnPeersChange(v.peers.GetPeers())
	}
}
