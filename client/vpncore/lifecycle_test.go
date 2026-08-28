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
