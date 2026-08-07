package vpncore

import (
	"encoding/binary"
	"fmt"
	"net"
	"net/url"
	"strconv"
	"sync"
	"time"
)

const (
	relayMagic      = 0x454e540a
	relayTypeReg    = 0x01
	relayTypeDat    = 0x02
	relayPort       = 3478
	relayRegTimeout = 30 * time.Second
)

type RelayClient struct {
	serverHost string
	conn       *net.UDPConn
	virtualIP  string
	token      string
	relayAddr  *net.UDPAddr
	mu         sync.Mutex
	started    bool
	stop       chan struct{}
	logFn      func(string, ...interface{})
}

func relayHostFromServerAddr(serverAddr string) string {
	host := serverAddr
	if u, err := url.Parse(serverAddr); err == nil && u.Host != "" {
		host = u.Host
	}
	if h, _, err := net.SplitHostPort(host); err == nil {
		return h
	}
	return host
}

func NewRelayClient(serverAddr string) *RelayClient {
	host := relayHostFromServerAddr(serverAddr)
	relayUDPAddr, _ := net.ResolveUDPAddr("udp4", net.JoinHostPort(host, strconv.Itoa(relayPort)))
	return &RelayClient{
		serverHost: host,
		relayAddr:  relayUDPAddr,
		stop:       make(chan struct{}),
	}
}

func (r *RelayClient) SetLogger(logFn func(string, ...interface{})) {
	r.logFn = logFn
}

func (r *RelayClient) logf(format string, args ...interface{}) {
	if r.logFn != nil {
		r.logFn(format, args...)
	}
}

func (r *RelayClient) SetToken(token string) {
	r.mu.Lock()
	r.token = token
	r.mu.Unlock()
}

func (r *RelayClient) Start(vip string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.started {
		return nil
	}
	r.virtualIP = vip
	r.started = true
	r.stop = make(chan struct{})
	go r.registerLoop()
	r.logf("Relay client started for %s via %s:%d", vip, r.serverHost, relayPort)
	return nil
}

func (r *RelayClient) Stop() {
	r.mu.Lock()
	defer r.mu.Unlock()
	if !r.started {
		return
	}
	select {
	case <-r.stop:
	default:
		close(r.stop)
	}
	r.started = false
	r.logf("Relay client stopped")
}

func (r *RelayClient) SetConn(conn *net.UDPConn) {
	r.mu.Lock()
	r.conn = conn
	r.mu.Unlock()
}

func (r *RelayClient) RelayAddr() *net.UDPAddr {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.relayAddr
}

func (r *RelayClient) SendToPeer(destVIP string, data []byte) error {
	r.mu.Lock()
	conn := r.conn
	relayAddr := r.relayAddr
	srcVIP := r.virtualIP
	r.mu.Unlock()

	if conn == nil || relayAddr == nil || srcVIP == "" {
		return fmt.Errorf("relay not ready")
	}

	// Format: [magic:4][type:1][dst_vip_len:1][dst_vip][src_vip_len:1][src_vip][payload]
	pkt := make([]byte, 5+1+len(destVIP)+1+len(srcVIP)+len(data))
	binary.BigEndian.PutUint32(pkt[0:4], relayMagic)
	pkt[4] = relayTypeDat
	pkt[5] = byte(len(destVIP))
	copy(pkt[6:], destVIP)
	off := 6 + len(destVIP)
	pkt[off] = byte(len(srcVIP))
	off++
	copy(pkt[off:], srcVIP)
	off += len(srcVIP)
	copy(pkt[off:], data)

	_, err := conn.WriteToUDP(pkt, relayAddr)
	return err
}

func (r *RelayClient) register() error {
	r.mu.Lock()
	conn := r.conn
	relayAddr := r.relayAddr
	vip := r.virtualIP
	token := r.token
	r.mu.Unlock()

	if conn == nil || relayAddr == nil {
		return fmt.Errorf("relay not ready")
	}
	if vip == "" {
		return fmt.Errorf("relay vip empty")
	}
	if token == "" {
		return fmt.Errorf("relay token empty")
	}

	pkt := make([]byte, 5+1+len(token)+len(vip))
	binary.BigEndian.PutUint32(pkt[0:4], relayMagic)
	pkt[4] = relayTypeReg
	pkt[5] = byte(len(token))
	copy(pkt[6:], token)
	copy(pkt[6+len(token):], vip)
	_, err := conn.WriteToUDP(pkt, relayAddr)
	return err
}

func (r *RelayClient) RegisterNow() {
	go func() {
		if err := r.register(); err != nil {
			r.logf("Relay re-register error: %v", err)
		} else {
			r.logf("Relay re-registration sent for %s", r.virtualIP)
		}
	}()
}

func (r *RelayClient) registerLoop() {
	// Check stop channel before initial sleep
	select {
	case <-r.stop:
		return
	case <-time.After(500 * time.Millisecond):
	}
	if err := r.register(); err != nil {
		r.logf("Relay registration error: %v", err)
	} else {
		r.logf("Relay registration sent for %s", r.virtualIP)
	}

	ticker := time.NewTicker(relayRegTimeout)
	defer ticker.Stop()

	for {
		select {
		case <-r.stop:
			return
		case <-ticker.C:
			if err := r.register(); err != nil {
				r.logf("Relay re-register error: %v", err)
			}
		}
	}
}
