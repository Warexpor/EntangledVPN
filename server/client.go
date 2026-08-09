package main

import (
	"crypto/subtle"
	"encoding/json"
	"log"
	"net"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

type Client struct {
	ID       string
	Nickname string
	Hub      *Hub
	Conn     *websocket.Conn
	Send     chan []byte
	Room     *Room
	Authed   bool

	PublicIP     net.IP
	PublicPort   int
	LocalAddr    string
	PublicKey    string
	VirtualIP    string
	ExternalAddr string
	Crypto       string

	mu sync.RWMutex
}

func NewClient(id, nickname string, conn *websocket.Conn, hub *Hub) *Client {
	return &Client{
		ID:       id,
		Nickname: nickname,
		Conn:     conn,
		Hub:      hub,
		Send:     make(chan []byte, 256),
	}
}

func (c *Client) remoteIP() string {
	if tcpAddr, ok := c.Conn.RemoteAddr().(*net.TCPAddr); ok {
		return tcpAddr.IP.String()
	}
	return c.Conn.RemoteAddr().String()
}

func (c *Client) ReadPump() {
	defer func() {
		c.Hub.Unregister <- c
		c.Conn.Close()
	}()

	for {
		_, msg, err := c.Conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseNormalClosure) {
				log.Printf("read error: %v", err)
			}
			break
		}
		c.handleMessage(msg)
	}
}

func (c *Client) WritePump() {
	ticker := time.NewTicker(30 * time.Second)
	defer func() {
		ticker.Stop()
		c.Conn.Close()
	}()

	for {
		select {
		case msg, ok := <-c.Send:
			if !ok {
				c.Conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}
			c.Conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
			if err := c.Conn.WriteMessage(websocket.TextMessage, msg); err != nil {
				log.Printf("write error to %s: %v", c.Nickname, err)
				return
			}
		case <-ticker.C:
			c.Conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
			if err := c.Conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}

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
	PublicKey    string `json:"public_key"`
	ExternalAddr string `json:"external_addr,omitempty"`
	Crypto       string `json:"crypto,omitempty"`
}

func (c *Client) handleMessage(raw []byte) {
	var msg Message
	if err := json.Unmarshal(raw, &msg); err != nil {
		log.Printf("invalid message from %s", c.Nickname)
		return
	}

	log.Printf("recv from %s [%s]: type=%s", c.Nickname, c.ID, msg.Type)

	switch msg.Type {
	case "auth":
		ip := c.remoteIP()
		if !c.Hub.allowRate(ip, "auth", 20, time.Minute) {
			c.sendError("rate limited")
			return
		}
		var p AuthPayload
		if err := json.Unmarshal(msg.Payload, &p); err != nil {
			c.sendError("invalid auth payload")
			return
		}
		if c.Hub.AuthToken != "" {
			if subtle.ConstantTimeCompare([]byte(c.Hub.AuthToken), []byte(p.Token)) != 1 {
				c.sendError("invalid server token")
				return
			}
		}
		if p.Nickname != "" {
			c.Nickname = p.Nickname
		}
		if tcpAddr, ok := c.Conn.RemoteAddr().(*net.TCPAddr); ok {
			c.PublicIP = net.ParseIP(tcpAddr.IP.String())
			c.PublicPort = tcpAddr.Port
		}
		c.Authed = true
		c.sendMessage("ok", mustMarshal(map[string]string{"message": "authenticated", "id": c.ID}))

	case "create_room":
		if !c.Authed {
			c.sendError("not authenticated")
			return
		}
		if !c.Hub.allowRate(c.remoteIP(), "create", 10, time.Minute) {
			c.sendError("rate limited")
			return
		}
		var p CreateRoomPayload
		if err := json.Unmarshal(msg.Payload, &p); err != nil {
			c.sendError("invalid payload")
			return
		}
		c.Hub.CreateRoom(p.Name, p.Password, c)

	case "join_room":
		if !c.Authed {
			c.sendError("not authenticated")
			return
		}
		if !c.Hub.allowRate(c.remoteIP(), "join", 30, time.Minute) {
			c.sendError("rate limited")
			return
		}
		var p JoinRoomPayload
		if err := json.Unmarshal(msg.Payload, &p); err != nil {
			c.sendError("invalid payload")
			return
		}
		c.Hub.JoinRoom(p.Name, p.Password, p.OwnerToken, c)

	case "leave_room":
		if !c.Authed {
			c.sendError("not authenticated")
			return
		}
		c.Hub.LeaveRoom(c)

	case "peer_info":
		if !c.Authed {
			c.sendError("not authenticated")
			return
		}
		var p PeerInfoPayload
		if err := json.Unmarshal(msg.Payload, &p); err != nil {
			c.sendError("invalid peer info")
			return
		}
		c.mu.Lock()
		// Bind pubkey once per session — prevents mid-session spoof swaps.
		if c.PublicKey == "" && p.PublicKey != "" {
			c.PublicKey = p.PublicKey
		}
		c.LocalAddr = p.LocalAddr
		c.ExternalAddr = p.ExternalAddr
		c.Crypto = p.Crypto
		room := c.Room
		vip := c.VirtualIP
		pubIP := c.PublicIP
		pubKey := c.PublicKey
		crypto := c.Crypto
		c.mu.Unlock()

		if room != nil {
			publicAddr := choosePublicAddr(pubIP, p.ExternalAddr, p.LocalAddr)
			payload := map[string]string{
				"id":          c.ID,
				"virtual_ip":  vip,
				"local_addr":  p.LocalAddr,
				"public_addr": publicAddr,
				"public_key":  pubKey,
			}
			if crypto != "" {
				payload["crypto"] = crypto
			}
			room.BroadcastExcept(c, Message{
				Type:    "peer_updated",
				Payload: mustMarshal(payload),
			})
		}

	case "relay_data":
		if !c.Authed {
			c.sendError("not authenticated")
			return
		}
		var p struct {
			To string `json:"to"`
			D  string `json:"d"`
		}
		if err := json.Unmarshal(msg.Payload, &p); err != nil || p.To == "" || p.D == "" {
			c.sendError("invalid relay_data payload")
			return
		}
		c.mu.RLock()
		room := c.Room
		c.mu.RUnlock()
		if room == nil || !room.HasClient(p.To) {
			c.sendError("relay target not in room")
			return
		}
		c.Hub.mu.Lock()
		target, exists := c.Hub.Clients[p.To]
		c.Hub.mu.Unlock()
		if exists {
			target.sendMessage("relay_data", mustMarshal(map[string]string{
				"from": c.ID,
				"d":    p.D,
			}))
		}

	case "delete_room":
		if !c.Authed {
			c.sendError("not authenticated")
			return
		}
		var p struct {
			Name       string `json:"name"`
			OwnerToken string `json:"owner_token,omitempty"`
		}
		if err := json.Unmarshal(msg.Payload, &p); err != nil || p.Name == "" {
			c.sendError("invalid delete_room payload")
			return
		}
		c.Hub.DeleteRoom(p.Name, p.OwnerToken, c)

	default:
		c.sendError("unknown message type")
	}
}

func (c *Client) sendError(msg string) {
	c.sendMessage("error", mustMarshal(map[string]string{"message": msg}))
}

func (c *Client) sendMessage(msgType string, payload json.RawMessage) {
	data, err := json.Marshal(Message{Type: msgType, Payload: payload})
	if err != nil {
		log.Printf("sendMessage marshal error for %s: %v", c.Nickname, err)
		return
	}
	select {
	case c.Send <- data:
	default:
		log.Printf("sendMessage: buffer full for %s (%s), dropping message type %s", c.Nickname, c.ID, msgType)
	}
}

func mustMarshal(v interface{}) json.RawMessage {
	data, _ := json.Marshal(v)
	return data
}

func choosePublicAddr(publicIP net.IP, externalAddr, localAddr string) string {
	if externalAddr != "" {
		return externalAddr
	}
	if publicIP == nil || localAddr == "" {
		return ""
	}
	_, port, err := net.SplitHostPort(localAddr)
	if err != nil {
		return ""
	}
	return net.JoinHostPort(publicIP.String(), port)
}
