package vpncore

import (
	"net"
	"testing"
	"time"
)

func TestDiscoverPublicAddrFallbackCloudflare(t *testing.T) {
	conn, err := net.ListenUDP("udp4", &net.UDPAddr{IP: net.IPv4zero, Port: 0})
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()

	ea, used, err := DiscoverPublicAddrFallback("", 8*time.Second, conn)
	if err != nil {
		t.Skipf("STUN unreachable in this environment: %v", err)
	}
	if ea == "" {
		t.Fatal("empty mapped addr")
	}
	host, port, err := net.SplitHostPort(ea)
	if err != nil || host == "" || port == "0" {
		t.Fatalf("bad mapped addr %q: %v", ea, err)
	}
	t.Logf("mapped %s via %s", ea, used)
}
