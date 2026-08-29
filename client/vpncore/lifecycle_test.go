package vpncore

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/gorilla/websocket"
)

func TestRoomOperationsRequireConnection(t *testing.T) {
	v := NewVPNCore(&VPNConfig{})

	if err := v.CreateRoom("demo", ""); err == nil {
		t.Fatal("CreateRoom should fail when signaling is unavailable")
	}
	if err := v.JoinRoom("demo", ""); err == nil {
		t.Fatal("JoinRoom should fail when signaling is unavailable")
	}
}

func TestTUNAdapterCanClearDNS(t *testing.T) {
	tun := NewTUNAdapter()
	tun.SetDNS("1.1.1.1")
	tun.SetDNS("")

	if tun.dnsServer != "" {
		t.Fatalf("expected DNS to be cleared, got %q", tun.dnsServer)
	}
}

func TestStartAuthFailureClosesSignaling(t *testing.T) {
	var live atomic.Int32
	upgrader := websocket.Upgrader{CheckOrigin: func(r *http.Request) bool { return true }}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/ws" {
			http.NotFound(w, r)
			return
		}
		c, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			return
		}
		live.Add(1)
		defer live.Add(-1)
		defer c.Close()

		_, _, _ = c.ReadMessage()
		payload, _ := json.Marshal(map[string]string{"message": "unauthorized"})
		_ = c.WriteJSON(Message{Type: "error", Payload: payload})
		for {
			if _, _, err := c.ReadMessage(); err != nil {
				return
			}
		}
	}))
	defer func() {
		closed := make(chan struct{})
		go func() {
			srv.Close()
			close(closed)
		}()
		select {
		case <-closed:
		case <-time.After(2 * time.Second):
			t.Fatal("httptest server still has a live WebSocket after Start failure")
		}
	}()

	addr := strings.TrimPrefix(srv.URL, "http://")
	v := NewVPNCore(&VPNConfig{ServerAddr: addr, Nickname: "tester"})
	err := v.Start()
	if err == nil {
		t.Fatal("Start should fail when signaling rejects auth")
	}

	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if !signalingConnLive(v) && live.Load() == 0 {
			break
		}
		time.Sleep(20 * time.Millisecond)
	}
	if signalingConnLive(v) {
		t.Fatal("Start auth failure left a usable live signaling client")
	}
	if live.Load() != 0 {
		t.Fatalf("server still has %d live WebSocket connection(s)", live.Load())
	}
	if err := v.CreateRoom("demo", ""); err == nil {
		t.Fatal("CreateRoom should fail after Start auth failure")
	}
	if err := v.JoinRoom("demo", ""); err == nil {
		t.Fatal("JoinRoom should fail after Start auth failure")
	}
}

func TestStopAfterFailedDialIsSafe(t *testing.T) {
	v := NewVPNCore(&VPNConfig{ServerAddr: "127.0.0.1:1", Nickname: "tester"})
	if err := v.Start(); err == nil {
		t.Fatal("Start should fail when the server is unreachable")
	}
	v.Stop()
	v.Stop()
	if signalingConnLive(v) {
		t.Fatal("Stop after failed Start left a live signaling client")
	}
	if err := v.CreateRoom("demo", ""); err == nil {
		t.Fatal("CreateRoom should fail when not connected")
	}
	if err := v.JoinRoom("demo", ""); err == nil {
		t.Fatal("JoinRoom should fail when not connected")
	}
}

func signalingConnLive(v *VPNCore) bool {
	v.mu.Lock()
	sig := v.signaling
	v.mu.Unlock()
	if sig == nil {
		return false
	}
	sig.mu.Lock()
	defer sig.mu.Unlock()
	return sig.conn != nil
}
