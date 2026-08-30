//go:build windows

package vpncore

import (
	_ "embed"
	"fmt"
	"log"
	"net"
	"os"
	"path/filepath"
	"strings"
	"time"

	"golang.zx2c4.com/wintun"
)

//go:embed native/wintun.dll
var embeddedWintunDLL []byte

func init() {
	ensureWintunDLL()
}

// ensureWintunDLL writes the embedded driver next to the exe if missing.
// Consumers only download Entangled.exe; first run creates wintun.dll beside it.
func ensureWintunDLL() {
	exe, err := os.Executable()
	if err != nil {
		return
	}
	dllPath := filepath.Join(filepath.Dir(exe), "wintun.dll")
	if _, err := os.Stat(dllPath); err == nil {
		return
	}
	if len(embeddedWintunDLL) == 0 {
		log.Printf("Warning: embedded wintun.dll is empty")
		return
	}
	if err := os.WriteFile(dllPath, embeddedWintunDLL, 0644); err != nil {
		log.Printf("Warning: failed to write wintun.dll to %s: %v", dllPath, err)
		return
	}
	log.Printf("Extracted embedded wintun.dll to %s", dllPath)
}

const (
	WINTUN_NAME = "Entangled"
	WINTUN_TYPE = "Entangled VPN"
)

type TUNAdapter struct {
	adapter   *wintun.Adapter
	session   wintun.Session
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
	if t.adapter != nil {
		t.logf("TUN already started, updating IP to %s", ip)
		return t.setInterfaceIP(ip)
	}

	t.logf("TUN Start: opening adapter")
	adapter, err := wintun.OpenAdapter(WINTUN_NAME)
	if err != nil {
		t.logf("TUN OpenAdapter failed: %v, trying CreateAdapter", err)
		adapter, err = wintun.CreateAdapter(WINTUN_NAME, WINTUN_TYPE, nil)
		if err != nil {
			return fmt.Errorf("create wintun adapter: %v", err)
		}
		t.logf("TUN CreateAdapter succeeded")
	} else {
		t.logf("TUN OpenAdapter succeeded")
	}
	t.adapter = adapter

	t.logf("TUN StartSession")
	session, err := adapter.StartSession(0x800000)
	if err != nil {
		adapter.Close()
		t.adapter = nil
		return fmt.Errorf("start wintun session: %v", err)
	}
	t.session = session

	t.logf("Wintun adapter started with IP %s", ip)
	if err := t.setInterfaceIP(ip); err != nil {
		t.logf("Warning: failed to set IP on interface: %v", err)
	}
	go t.readLoop()
	return nil
}

func (t *TUNAdapter) readLoop() {
	stop := t.stop
	for {
		// Check if we should stop before blocking on ReceivePacket
		select {
		case <-stop:
			return
		default:
		}

		packet, err := t.session.ReceivePacket()
		if err != nil {
			select {
			case <-stop:
				return
			default:
				time.Sleep(10 * time.Millisecond)
			}
			continue
		}

		n := len(packet)
		if n == 0 {
			t.session.ReleaseReceivePacket(packet)
			continue
		}

		pkt := make([]byte, n)
		copy(pkt, packet)
		t.session.ReleaseReceivePacket(packet)

		if t.OnPacket != nil {
			t.OnPacket(pkt)
		}
	}
}

func (t *TUNAdapter) setInterfaceIP(ip string) error {
	// First, remove any existing IP config on this interface
	HiddenCommand("netsh", "interface", "ip", "set", "address",
		"name="+WINTUN_NAME,
		"source=dhcp").Run()

	// Set as /32 point-to-point (no subnet route auto-created)
	args := []string{
		"interface", "ip", "set", "address",
		"name=" + WINTUN_NAME,
		"source=static",
		fmt.Sprintf("addr=%s", ip),
		"mask=255.255.255.255",
		"gateway=none",
	}
	cmd := HiddenCommand("netsh", args...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("netsh failed: %v, output: %s", err, strings.TrimSpace(string(out)))
	}
	t.logf("Interface IP configured: %s/32", ip)

	// Set low metric
	HiddenCommand("netsh", "interface", "ip", "set", "interface",
		fmt.Sprintf("name=%s", WINTUN_NAME),
		"metric=1").Run()

	// Apply configured MTU after IP is set
	if t.MTU > 0 && t.MTU != 1500 {
		t.applyMTU()
	}

	// Apply configured DNS after IP is set
	if t.dnsServer != "" {
		t.applyDNS()
	}

	// Add a route for the VPN subnet through this interface
	local := net.ParseIP(ip).To4()
	if local != nil {
		subnet := fmt.Sprintf("%d.%d.%d.0/24", local[0], local[1], local[2])
		addRouteCmd := HiddenCommand("netsh", "interface", "ip", "add", "route",
			fmt.Sprintf("prefix=%s", subnet),
			fmt.Sprintf("interface=%s", WINTUN_NAME),
			"metric=256",
		)
		if routeOut, routeErr := addRouteCmd.CombinedOutput(); routeErr != nil {
			t.logf("Warning: failed to add subnet route: %v, output: %s", routeErr, strings.TrimSpace(string(routeOut)))
		} else {
			t.logf("Added route %s -> %s", subnet, WINTUN_NAME)
		}
	}
	return nil
}

func (t *TUNAdapter) AddRoute(dstIP string) error {
	cmd := HiddenCommand("netsh", "interface", "ip", "add", "route",
		fmt.Sprintf("prefix=%s/32", dstIP),
		fmt.Sprintf("interface=%s", WINTUN_NAME),
		"metric=256",
	)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("add route %s failed: %v, output: %s", dstIP, err, strings.TrimSpace(string(out)))
	}
	t.logf("Added peer route %s -> %s", dstIP, WINTUN_NAME)
	return nil
}

func (t *TUNAdapter) RemoveRoute(dstIP string) error {
	cmd := HiddenCommand("netsh", "interface", "ip", "delete", "route",
		fmt.Sprintf("prefix=%s/32", dstIP),
		fmt.Sprintf("interface=%s", WINTUN_NAME),
	)
	cmd.Run()
	t.logf("Removed peer route %s", dstIP)
	return nil
}

func (t *TUNAdapter) Write(data []byte) (int, error) {
	if t.adapter == nil {
		return 0, fmt.Errorf("tun closed")
	}
	packet, err := t.session.AllocateSendPacket(len(data))
	if err != nil || packet == nil {
		return 0, err
	}
	copy(packet, data)
	t.session.SendPacket(packet)
	return len(data), nil
}

// SetMTU configures the TUN adapter MTU and applies it via netsh.
func (t *TUNAdapter) SetMTU(mtu int) {
	if mtu <= 0 {
		mtu = 1500
	}
	t.MTU = mtu
	t.applyMTU()
}

func (t *TUNAdapter) applyMTU() {
	cmd := HiddenCommand("netsh", "interface", "ipv4", "set", "subinterface",
		fmt.Sprintf("name=%s", WINTUN_NAME),
		fmt.Sprintf("mtu=%d", t.MTU),
		"store=persistent",
	)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.logf("Warning: failed to set MTU to %d: %v, output: %s", t.MTU, err, strings.TrimSpace(string(out)))
	} else {
		t.logf("MTU set to %d", t.MTU)
	}
}

// SetDNS configures a static DNS server for the TUN adapter.
func (t *TUNAdapter) SetDNS(dnsServer string) {
	t.dnsServer = dnsServer
	t.applyDNS()
}

func (t *TUNAdapter) applyDNS() {
	if t.dnsServer == "" {
		cmd := HiddenCommand("netsh", "interface", "ip", "set", "dns",
			fmt.Sprintf("name=%s", WINTUN_NAME),
			"source=dhcp",
		)
		if out, err := cmd.CombinedOutput(); err != nil {
			t.logf("Warning: failed to clear DNS: %v, output: %s", err, strings.TrimSpace(string(out)))
		} else {
			t.logf("DNS cleared (DHCP)")
		}
		return
	}
	cmd := HiddenCommand("netsh", "interface", "ip", "set", "dns",
		fmt.Sprintf("name=%s", WINTUN_NAME),
		"source=static",
		fmt.Sprintf("addr=%s", t.dnsServer),
		"register=primary",
	)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.logf("Warning: failed to set DNS to %s: %v, output: %s", t.dnsServer, err, strings.TrimSpace(string(out)))
	} else {
		t.logf("DNS set to %s", t.dnsServer)
	}
}

func (t *TUNAdapter) Close() {
	select {
	case <-t.stop:
	default:
		close(t.stop)
	}
	if t.adapter != nil {
		t.session.End()
		t.adapter.Close()
		t.adapter = nil
		t.session = wintun.Session{}
	}
	t.stop = make(chan struct{})
}
