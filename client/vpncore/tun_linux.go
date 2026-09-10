//go:build linux

package vpncore

import (
	"fmt"
	"net"
	"os/exec"
	"strings"
	"sync"

	"golang.zx2c4.com/wireguard/tun"
)

// Keep names aligned with the Windows Wintun adapter for shared vpncore call sites.
const (
	WINTUN_NAME = "entangled"
	WINTUN_TYPE = "Entangled VPN"
)

// Packet buffers reserve room for virtio-net headers used by wireguard/tun on Linux.
const tunPacketOffset = 16

type TUNAdapter struct {
	device    tun.Device
	name      string
	MTU       int
	dnsServer string
	OnPacket  func([]byte)
	OnLog     func(string, ...interface{})
	stop chan struct{}
	mu   sync.Mutex
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
	t.mu.Lock()
	defer t.mu.Unlock()

	if t.device != nil {
		t.logf("TUN already started, updating IP to %s", ip)
		return t.setInterfaceIP(ip)
	}

	mtu := t.MTU
	if mtu <= 0 {
		mtu = 1500
	}

	t.logf("TUN Start: creating device %s mtu=%d", WINTUN_NAME, mtu)
	dev, err := tun.CreateTUN(WINTUN_NAME, mtu)
	if err != nil {
		return fmt.Errorf("create tun %s: %w (need CAP_NET_ADMIN / root)", WINTUN_NAME, err)
	}
	name, err := dev.Name()
	if err != nil {
		dev.Close()
		return fmt.Errorf("tun name: %w", err)
	}
	t.device = dev
	t.name = name
	t.stop = make(chan struct{})

	if err := t.setInterfaceIP(ip); err != nil {
		t.logf("Warning: failed to set IP on interface: %v", err)
	}
	go t.readLoop()
	t.logf("TUN adapter started name=%s ip=%s", name, ip)
	return nil
}

func (t *TUNAdapter) readLoop() {
	dev := t.device
	if dev == nil {
		return
	}
	stop := t.stop
	buf := make([]byte, tunPacketOffset+65535)
	bufs := [][]byte{buf}
	sizes := make([]int, 1)

	for {
		select {
		case <-stop:
			return
		default:
		}

		n, err := dev.Read(bufs, sizes, tunPacketOffset)
		if err != nil {
			select {
			case <-stop:
				return
			default:
				t.logf("TUN read error: %v", err)
				return
			}
		}
		for i := 0; i < n; i++ {
			sz := sizes[i]
			if sz <= 0 {
				continue
			}
			pkt := make([]byte, sz)
			copy(pkt, bufs[i][tunPacketOffset:tunPacketOffset+sz])
			if t.OnPacket != nil {
				t.OnPacket(pkt)
			}
		}
	}
}

func (t *TUNAdapter) ifName() string {
	if t.name != "" {
		return t.name
	}
	return WINTUN_NAME
}

func (t *TUNAdapter) runIP(args ...string) error {
	cmd := exec.Command("ip", args...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("ip %s: %v (%s)", strings.Join(args, " "), err, strings.TrimSpace(string(out)))
	}
	return nil
}

func (t *TUNAdapter) setInterfaceIP(ip string) error {
	name := t.ifName()
	_ = t.runIP("link", "set", "dev", name, "up")
	_ = exec.Command("ip", "addr", "flush", "dev", name).Run()

	if err := t.runIP("addr", "add", ip+"/32", "dev", name); err != nil {
		// Idempotent-ish: ignore "file exists"
		if !strings.Contains(err.Error(), "File exists") && !strings.Contains(err.Error(), "file exists") {
			return err
		}
	}
	t.logf("Interface IP configured: %s/32 on %s", ip, name)

	if t.MTU > 0 {
		t.applyMTULocked()
	}
	if t.dnsServer != "" {
		t.applyDNSLocked()
	}

	local := net.ParseIP(ip).To4()
	if local != nil {
		subnet := fmt.Sprintf("%d.%d.%d.0/24", local[0], local[1], local[2])
		_ = exec.Command("ip", "route", "replace", subnet, "dev", name).Run()
		t.logf("Added route %s -> %s", subnet, name)
	}
	return nil
}

func (t *TUNAdapter) AddRoute(dstIP string) error {
	t.mu.Lock()
	defer t.mu.Unlock()
	name := t.ifName()
	if err := t.runIP("route", "replace", dstIP+"/32", "dev", name); err != nil {
		return fmt.Errorf("add route %s failed: %w", dstIP, err)
	}
	t.logf("Added peer route %s -> %s", dstIP, name)
	return nil
}

func (t *TUNAdapter) RemoveRoute(dstIP string) error {
	t.mu.Lock()
	defer t.mu.Unlock()
	name := t.ifName()
	_ = exec.Command("ip", "route", "del", dstIP+"/32", "dev", name).Run()
	t.logf("Removed peer route %s", dstIP)
	return nil
}

func (t *TUNAdapter) Write(data []byte) (int, error) {
	t.mu.Lock()
	dev := t.device
	t.mu.Unlock()
	if dev == nil {
		return 0, fmt.Errorf("tun closed")
	}
	buf := make([]byte, tunPacketOffset+len(data))
	copy(buf[tunPacketOffset:], data)
	_, err := dev.Write([][]byte{buf}, tunPacketOffset)
	if err != nil {
		return 0, err
	}
	return len(data), nil
}

func (t *TUNAdapter) SetMTU(mtu int) {
	t.mu.Lock()
	defer t.mu.Unlock()
	if mtu <= 0 {
		mtu = 1500
	}
	t.MTU = mtu
	t.applyMTULocked()
}

func (t *TUNAdapter) applyMTULocked() {
	if t.device == nil {
		return
	}
	name := t.ifName()
	if err := t.runIP("link", "set", "dev", name, "mtu", fmt.Sprintf("%d", t.MTU)); err != nil {
		t.logf("Warning: failed to set MTU to %d: %v", t.MTU, err)
	} else {
		t.logf("MTU set to %d", t.MTU)
	}
}

func (t *TUNAdapter) SetDNS(dnsServer string) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.dnsServer = dnsServer
	t.applyDNSLocked()
}

func (t *TUNAdapter) applyDNSLocked() {
	if t.device == nil {
		return
	}
	name := t.ifName()
	// Best-effort via systemd-resolved; ignore failures on systems without it.
	if t.dnsServer == "" {
		out, err := exec.Command("resolvectl", "dns", name, "").CombinedOutput()
		if err != nil {
			t.logf("Warning: failed to clear DNS via resolvectl: %v (%s)", err, strings.TrimSpace(string(out)))
		} else {
			t.logf("DNS cleared")
		}
		return
	}
	out, err := exec.Command("resolvectl", "dns", name, t.dnsServer).CombinedOutput()
	if err != nil {
		t.logf("Warning: failed to set DNS to %s: %v (%s)", t.dnsServer, err, strings.TrimSpace(string(out)))
	} else {
		t.logf("DNS set to %s", t.dnsServer)
	}
}

func (t *TUNAdapter) Close() {
	t.mu.Lock()
	defer t.mu.Unlock()
	select {
	case <-t.stop:
	default:
		close(t.stop)
	}
	if t.device != nil {
		_ = t.device.Close()
		t.device = nil
	}
	t.name = ""
	t.stop = make(chan struct{})
}
