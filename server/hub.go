package main

import (
	"crypto/rand"
	"encoding/hex"
	"log"
	"net/http"
	"net/url"
	"os"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		origin := r.Header.Get("Origin")
		if origin == "" {
			return true
		}
		u, err := url.Parse(origin)
		if err != nil {
			log.Printf("WebSocket origin check rejected (parse error): origin=%s host=%s err=%v", origin, r.Host, err)
			return false
		}
		if u.Host == r.Host {
			return true
		}
		log.Printf("WebSocket origin check rejected: origin=%s host=%s", origin, r.Host)
		return false
	},
}

type Hub struct {
	Clients     map[string]*Client
	Rooms       map[string]*Room
	RelayTokens map[string]*RelayTokenEntry
	Register    chan *Client
	Unregister  chan *Client
	mu          sync.Mutex
	Relay       *Relay
	AuthToken   string
	rate        map[string][]time.Time
}

func NewHub() *Hub {
	return &Hub{
		Clients:     make(map[string]*Client),
		Rooms:       make(map[string]*Room),
		RelayTokens: make(map[string]*RelayTokenEntry),
		Register:    make(chan *Client),
		Unregister:  make(chan *Client),
		AuthToken:   os.Getenv("ENTANGLED_TOKEN"),
		rate:        make(map[string][]time.Time),
	}
}

func (h *Hub) Run() {
	go h.cleanupTokens()
	for {
		select {
		case client := <-h.Register:
			h.mu.Lock()
			h.Clients[client.ID] = client
			h.mu.Unlock()
			log.Printf("client connected: %s (%s)", client.Nickname, client.ID)

		case client := <-h.Unregister:
			h.mu.Lock()
			if _, ok := h.Clients[client.ID]; ok {
				h.leaveRoom(client)
				delete(h.Clients, client.ID)
				close(client.Send)
				log.Printf("client disconnected: %s", client.Nickname)
			}
			h.mu.Unlock()
		}
	}
}

func (h *Hub) cleanupTokens() {
	ticker := time.NewTicker(60 * time.Second)
	defer ticker.Stop()
	for range ticker.C {
		h.mu.Lock()
		now := time.Now()
		for k, e := range h.RelayTokens {
			if now.After(e.Expiry) {
				delete(h.RelayTokens, k)
			}
		}
		h.mu.Unlock()
	}
}

func (h *Hub) allowRate(ip, action string, limit int, window time.Duration) bool {
	key := ip + "|" + action
	h.mu.Lock()
	defer h.mu.Unlock()
	now := time.Now()
	cut := now.Add(-window)
	events := h.rate[key]
	kept := events[:0]
	for _, t := range events {
		if t.After(cut) {
			kept = append(kept, t)
		}
	}
	if len(kept) >= limit {
		h.rate[key] = kept
		return false
	}
	kept = append(kept, now)
	h.rate[key] = kept
	return true
}

func (h *Hub) HandleWS(w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("upgrade error: %v", err)
		return
	}

	id := generateID()
	client := NewClient(id, "User-"+id, conn, h)
	h.Register <- client

	go client.WritePump()
	go client.ReadPump()
}

func generateID() string {
	b := make([]byte, 6)
	if _, err := rand.Read(b); err != nil {
		return hex.EncodeToString([]byte(time.Now().Format("150405.000000")))
	}
	return hex.EncodeToString(b)
}
