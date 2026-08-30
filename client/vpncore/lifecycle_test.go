package vpncore

import "testing"
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
