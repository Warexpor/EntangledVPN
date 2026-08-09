package vpncore

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"golang.org/x/net/proxy"
)

type Message struct {
	Type    string          `json:"type"`
	Payload json.RawMessage `json:"payload,omitempty"`
}

type AuthPayload struct {
	Token    string `json:"token"`
	Nickname string `json:"nickname"`
	Version  string `json:"version"`
}

type CreateRoomPayload struct {
	Name     string `json:"name"`
	Password string `json:"password,omitempty"`
}

type JoinRoomPayload struct {
	Name       string `json:"name"`
	Password   string `json:"password,omitempty"`
	OwnerToken string `json:"owner_token,omitempty"`
}

type PeerInfoPayload struct {
	VirtualIP    string `json:"virtual_ip"`
	LocalAddr    string `json:"local_addr"`
	PublicAddr   string `json:"public_addr,omitempty"`
	PublicKey    string `json:"public_key"`
	ExternalAddr string `json:"external_addr,omitempty"`
	Crypto       string `json:"crypto,omitempty"`
}

type RoomJoinedPayload struct {
	Room       string              `json:"room"`
	IsOwner    bool                `json:"is_owner"`
	VirtualIP  string              `json:"virtual_ip"`
	PeerList   []map[string]string `json:"peer_list"`
	RelayToken string              `json:"relay_token,omitempty"`
	OwnerToken string              `json:"owner_token,omitempty"`
}

type SignalingClient struct {
	conn           *websocket.Conn
	mu             sync.Mutex
	done           chan struct{}
	closeOnce      sync.Once
	intentional    bool
	log            func(string, ...interface{})
	localID        string
	socks5Addr     string
	authToken      string
	OnLocalID      func(id string)
	OnPeerJoined   func(id, nickname string)
	OnPeerLeft     func(id string)
	OnPeerUpdated  func(id, virtualIP, localAddr, publicAddr, publicKey, crypto string)
	OnRoomJoined   func(room string, isOwner bool, virtualIP string, peers []map[string]string, relayToken, ownerToken string)
	OnRoomDeleted  func(name string)
	OnError        func(msg string)
	OnDisconnected func(intentional bool)
	OnRelayData    func(fromID string, data []byte)
}

func NewSignalingClient() *SignalingClient {
	return &SignalingClient{
		done: make(chan struct{}),
	}
}

func (s *SignalingClient) SetLogger(logFn func(string, ...interface{})) {
	s.log = logFn
}

func (s *SignalingClient) SetSOCKS5(addr string) {
	s.socks5Addr = addr
}

func (s *SignalingClient) SetAuthToken(token string) {
	s.authToken = token
}

func (s *SignalingClient) logf(format string, args ...interface{}) {
	if s.log != nil {
		s.log(format, args...)
	}
}

// normalizeWSAddr picks ws/wss and a bare host[:port].
// https://host:8080 → ws (plain servers; browser URL habit). Explicit wss:// always TLS.
func normalizeWSAddr(serverAddr string) (scheme, host string) {
	raw := strings.TrimSpace(serverAddr)
	lower := strings.ToLower(raw)
	hadHTTPS, hadWSS, hadWS, hadHTTP := false, false, false, false
	for _, p := range []struct {
		prefix string
		flag   *bool
	}{
		{"https://", &hadHTTPS},
		{"http://", &hadHTTP},
		{"wss://", &hadWSS},
		{"ws://", &hadWS},
	} {
		if strings.HasPrefix(lower, p.prefix) {
			*p.flag = true
			raw = raw[len(p.prefix):]
			break
		}
	}
	if i := strings.IndexAny(raw, "/?"); i >= 0 {
		raw = raw[:i]
	}
	host = strings.TrimSuffix(raw, "/")

	_, port, splitErr := net.SplitHostPort(host)
	hasPort := splitErr == nil

	scheme = "ws"
	switch {
	case hadWSS:
		scheme = "wss"
	case hadWS, hadHTTP:
		scheme = "ws"
	case hadHTTPS:
		if !hasPort || port == "443" {
			scheme = "wss"
		}
	case hasPort && port == "443":
		scheme = "wss"
	}
	return scheme, host
}

func (s *SignalingClient) Connect(serverAddr, nickname string) error {
	scheme, cleanAddr := normalizeWSAddr(serverAddr)
	u := url.URL{Scheme: scheme, Host: cleanAddr, Path: "/ws"}
	s.logf("WebSocket dialing %s", u.String())
	// Do not inherit HTTP(S)_PROXY — env proxies often break plain ws:// to LAN/VPS hosts.
	// Explicit SOCKS5 (below) is the supported proxy path.
	dialer := websocket.Dialer{
		HandshakeTimeout: 15 * time.Second,
		Proxy:            nil,
	}

	if s.socks5Addr != "" {
		s.logf("Using SOCKS5 proxy: %s", s.socks5Addr)
		proxyDialer, err := proxy.SOCKS5("tcp", s.socks5Addr, nil, proxy.Direct)
		if err != nil {
			return fmt.Errorf("socks5 dialer: %v", err)
		}
		dialer.NetDial = proxyDialer.Dial
	}

	conn, _, err := dialer.Dial(u.String(), nil)
	if err != nil {
		if scheme == "wss" {
			return fmt.Errorf("ws dial %s: %v (TLS failed — for plain servers use host:port, not https://)", u.String(), err)
		}
		return fmt.Errorf("ws dial %s: %v", u.String(), err)
	}

	s.mu.Lock()
	s.conn = conn
	s.done = make(chan struct{})
	s.closeOnce = sync.Once{}
	s.intentional = false
	s.mu.Unlock()

	s.logf("WebSocket connected to %s", serverAddr)

	conn.SetPongHandler(func(appData string) error {
		s.mu.Lock()
		c := s.conn
		s.mu.Unlock()
		if c != nil {
			c.SetReadDeadline(time.Now().Add(60 * time.Second))
		}
		return nil
	})
	conn.SetReadDeadline(time.Now().Add(60 * time.Second))

	go s.readLoop()
	s.send(Message{Type: "auth", Payload: mustMarshalWithLog(s.logf, AuthPayload{
		Token:    s.authToken,
		Nickname: nickname,
		Version:  AppVersion,
	})})
	s.logf("Sent auth for %s", nickname)
	return nil
}

func (s *SignalingClient) readLoop() {
	pingTicker := time.NewTicker(20 * time.Second)
	defer func() {
		pingTicker.Stop()
		s.mu.Lock()
		intentional := s.intentional
		s.mu.Unlock()
		s.Close()
		s.logf("WebSocket readLoop ended (intentional=%v)", intentional)
		if s.OnDisconnected != nil {
			s.OnDisconnected(intentional)
		}
	}()

	go func() {
		for {
			select {
			case <-s.done:
				return
			case <-pingTicker.C:
				s.mu.Lock()
				if s.conn != nil {
					s.conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
					if err := s.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
						s.logf("Ping error: %v", err)
					}
				}
				s.mu.Unlock()
			}
		}
	}()

	for {
		s.mu.Lock()
		conn := s.conn
		s.mu.Unlock()
		if conn == nil {
			return
		}
		_, msg, err := conn.ReadMessage()
		if err != nil {
			s.logf("WebSocket read error: %v", err)
			return
		}

		var m Message
		if err := json.Unmarshal(msg, &m); err != nil {
			s.logf("JSON unmarshal error: %v", err)
			continue
		}

		s.logf("WS recv: type=%s", m.Type)
		s.handleMessage(m)
	}
}

func (s *SignalingClient) handleMessage(m Message) {
	switch m.Type {
	case "ok":
		var p struct {
			Message string `json:"message"`
			ID      string `json:"id"`
		}
		if json.Unmarshal(m.Payload, &p) == nil {
			s.logf("Server ok: %s (id=%s)", p.Message, p.ID)
			if p.ID != "" {
				s.localID = p.ID
				if s.OnLocalID != nil {
					s.OnLocalID(p.ID)
				}
			}
		}
	case "error":
		var p map[string]string
		json.Unmarshal(m.Payload, &p)
		errMsg := p["message"]
		s.logf("Server error: %s", errMsg)
		if s.OnError != nil {
			s.OnError(errMsg)
		}
	case "room_joined":
		var p RoomJoinedPayload
		if json.Unmarshal(m.Payload, &p) == nil {
			s.logf("Room joined: %s (owner=%v, vip=%s, peers=%d)", p.Room, p.IsOwner, p.VirtualIP, len(p.PeerList))
			if s.OnRoomJoined != nil {
				s.OnRoomJoined(p.Room, p.IsOwner, p.VirtualIP, p.PeerList, p.RelayToken, p.OwnerToken)
			}
		} else {
			s.logf("Failed to parse room_joined")
		}
	case "peer_joined":
		var p struct {
			ID       string `json:"id"`
			Nickname string `json:"nickname"`
		}
		if json.Unmarshal(m.Payload, &p) == nil {
			s.logf("Peer joined: %s (%s)", p.Nickname, p.ID)
			if s.OnPeerJoined != nil {
				s.OnPeerJoined(p.ID, p.Nickname)
			}
		}
	case "peer_left":
		var p struct {
			ID string `json:"id"`
		}
		if json.Unmarshal(m.Payload, &p) == nil {
			s.logf("Peer left: %s", p.ID)
			if s.OnPeerLeft != nil {
				s.OnPeerLeft(p.ID)
			}
		}
	case "peer_updated":
		var p struct {
			ID         string `json:"id"`
			VirtualIP  string `json:"virtual_ip"`
			LocalAddr  string `json:"local_addr"`
			PublicAddr string `json:"public_addr,omitempty"`
			PublicKey  string `json:"public_key"`
			Crypto     string `json:"crypto,omitempty"`
		}
		if json.Unmarshal(m.Payload, &p) == nil {
			s.logf("Peer updated: %s ip=%s local=%s public=%s crypto=%s", p.ID, p.VirtualIP, p.LocalAddr, p.PublicAddr, p.Crypto)
			if s.OnPeerUpdated != nil {
				s.OnPeerUpdated(p.ID, p.VirtualIP, p.LocalAddr, p.PublicAddr, p.PublicKey, p.Crypto)
			}
		}
	case "relay_data":
		var p struct {
			From string `json:"from"`
			D    string `json:"d"`
		}
		if json.Unmarshal(m.Payload, &p) == nil && p.D != "" {
			data, err := base64.StdEncoding.DecodeString(p.D)
			if err == nil && s.OnRelayData != nil {
				s.OnRelayData(p.From, data)
			}
		}
	case "room_deleted":
		var p struct {
			Name string `json:"name"`
		}
		json.Unmarshal(m.Payload, &p)
		s.logf("Room deleted: %s", p.Name)
		if s.OnRoomDeleted != nil {
			s.OnRoomDeleted(p.Name)
		}
	default:
		s.logf("Unknown message type: %s", m.Type)
	}
}

func (s *SignalingClient) send(m Message) {
	s.mu.Lock()
	defer s.mu.Unlock()
	data, err := json.Marshal(m)
	if err != nil {
		s.logf("JSON marshal error: %v", err)
		return
	}
	s.logf("WS send: type=%s", m.Type)
	if s.conn == nil {
		s.logf("Cannot send, connection is nil")
		return
	}
	s.conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
	if err := s.conn.WriteMessage(websocket.TextMessage, data); err != nil {
		s.logf("WebSocket write error: %v", err)
	}
}

func (s *SignalingClient) CreateRoom(name, password string) {
	s.logf("Sending create_room: %s", name)
	s.send(Message{Type: "create_room", Payload: mustMarshalWithLog(s.logf, CreateRoomPayload{Name: name, Password: password})})
}

func (s *SignalingClient) JoinRoom(name, password, ownerToken string) {
	s.logf("Sending join_room: %s", name)
	s.send(Message{Type: "join_room", Payload: mustMarshalWithLog(s.logf, JoinRoomPayload{Name: name, Password: password, OwnerToken: ownerToken})})
}

func (s *SignalingClient) LeaveRoom() {
	s.logf("Sending leave_room")
	s.send(Message{Type: "leave_room"})
}

func (s *SignalingClient) DeleteRoom(name, ownerToken string) {
	s.logf("Sending delete_room: %s", name)
	s.send(Message{Type: "delete_room", Payload: mustMarshalWithLog(s.logf, map[string]string{"name": name, "owner_token": ownerToken})})
}

func (s *SignalingClient) SendPeerInfo(virtualIP, localAddr, publicKey, externalAddr string) {
	s.logf("Sending peer_info: ip=%s addr=%s external=%s crypto=%s", virtualIP, localAddr, externalAddr, CryptoHKDF)
	s.send(Message{Type: "peer_info", Payload: mustMarshalWithLog(s.logf, PeerInfoPayload{
		VirtualIP: virtualIP, LocalAddr: localAddr, PublicKey: publicKey,
		ExternalAddr: externalAddr, Crypto: CryptoHKDF,
	})})
}

func (s *SignalingClient) SendRelayData(toID string, data []byte) {
	payload, err := json.Marshal(map[string]string{
		"to": toID,
		"d":  base64.StdEncoding.EncodeToString(data),
	})
	if err != nil {
		s.logf("SendRelayData marshal error: %v", err)
		return
	}
	s.send(Message{Type: "relay_data", Payload: payload})
}

func (s *SignalingClient) Close() {
	s.mu.Lock()
	s.intentional = true
	s.mu.Unlock()
	s.closeOnce.Do(func() {
		close(s.done)
		s.mu.Lock()
		if s.conn != nil {
			s.conn.Close()
			s.conn = nil
		}
		s.mu.Unlock()
	})
}

func mustMarshalWithLog(logFn func(string, ...interface{}), v interface{}) json.RawMessage {
	data, err := json.Marshal(v)
	if err != nil && logFn != nil {
		logFn("JSON marshal error: %v", err)
	}
	return data
}

func mustMarshal(v interface{}) json.RawMessage {
	data, _ := json.Marshal(v)
	return data
}
