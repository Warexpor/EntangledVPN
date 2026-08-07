//go:build !windows

package vpncore

import "fmt"

// Stub TUN for non-Windows (CI / unit tests). Real adapter is Windows+Wintun only.

const (
	WINTUN_NAME = "Entangled"
	WINTUN_TYPE = "Entangled VPN"
)

type TUNAdapter struct {
	MTU       int
	dnsServer string
	OnPacket  func([]byte)
	OnLog     func(string, ...interface{})
	stop      chan struct{}
}

func NewTUNAdapter() *TUNAdapter {
	return &TUNAdapter{
		MTU:  1500,
		stop: make(chan struct{}),
	}
}

func (t *TUNAdapter) logf(format string, args ...interface{}) {
	if t.OnLog != nil {
		t.OnLog(format, args...)
	}
}

func (t *TUNAdapter) Start(ip string) error {
	return fmt.Errorf("TUN/Wintun is Windows-only")
}

func (t *TUNAdapter) AddRoute(dstIP string) error {
	return fmt.Errorf("TUN/Wintun is Windows-only")
}

func (t *TUNAdapter) RemoveRoute(dstIP string) error {
	return nil
}

func (t *TUNAdapter) Write(data []byte) (int, error) {
	return 0, fmt.Errorf("tun closed")
}

func (t *TUNAdapter) SetMTU(mtu int) {
	if mtu <= 0 {
		mtu = 1500
	}
	t.MTU = mtu
}

func (t *TUNAdapter) SetDNS(dnsServer string) {
	t.dnsServer = dnsServer
}

func (t *TUNAdapter) Close() {
	select {
	case <-t.stop:
	default:
		close(t.stop)
	}
	t.stop = make(chan struct{})
}
