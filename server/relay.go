package main

import (
	"encoding/binary"
	"log"
	"net"
	"sync"
	"time"
)

const (
	relayMagic   = 0x454e540a
	relayTypeReg = 0x01
	relayTypeDat = 0x02
	relayTimeout = 120 * time.Second
)

type RelayEntry struct {
	VirtualIP string
	ClientID  string
	UDPAddr   *net.UDPAddr
	lastSeen  time.Time
}

type Relay struct {
	entries map[string]*RelayEntry
	byAddr  map[string]*RelayEntry
	conn    *net.UDPConn
	mu      sync.RWMutex
	done    chan struct{}
	running bool
	Hub     *Hub
}

func NewRelay() *Relay {
	return &Relay{
		entries: make(map[string]*RelayEntry),
		byAddr:  make(map[string]*RelayEntry),
		done:    make(chan struct{}),
	}
}

// parseRelayReg accepts tokenized REG only:
//
//	[magic:4][type:1][token_len:1][token][vip]
func (r *Relay) parseRelayReg(pkt []byte) (vip string, ok bool) {
	n := len(pkt)
	if n < 6 {
		return "", false
	}
	tokenLen := int(pkt[5])
	if tokenLen <= 0 || 6+tokenLen >= n {
		log.Printf("Relay REG rejected (token required)")
		return "", false
	}
	token := string(pkt[6 : 6+tokenLen])
	cand := string(pkt[6+tokenLen : n])
	if net.ParseIP(cand) == nil {
		log.Printf("Relay REG rejected (unparseable vip)")
		return "", false
	}
	if r.Hub == nil || !r.Hub.ValidateRelayReg(token, cand) {
		log.Printf("Relay REG rejected for vip=%s (bad/expired token)", cand)
		return "", false
	}
	return cand, true
}

func (r *Relay) Start(addr string) error {
	r.mu.Lock()
	if r.running {
		r.mu.Unlock()
		return nil
	}
	r.mu.Unlock()

	udpAddr, err := net.ResolveUDPAddr("udp4", addr)
	if err != nil {
		return err
	}
	conn, err := net.ListenUDP("udp4", udpAddr)
	if err != nil {
		return err
	}

	r.mu.Lock()
	if r.running {
		r.mu.Unlock()
		conn.Close()
		return nil
	}
	r.conn = conn
	r.running = true
	r.mu.Unlock()

	log.Printf("Relay listening on %s", addr)
	go r.readLoop()
	go r.cleanupLoop()
	return nil
}

func (r *Relay) Stop() {
	select {
	case <-r.done:
	default:
		close(r.done)
	}
	if r.conn != nil {
		r.conn.Close()
	}
}

func (r *Relay) readLoop() {
	buf := make([]byte, 65535)
	for {
		select {
		case <-r.done:
			return
		default:
		}

		n, addr, err := r.conn.ReadFromUDP(buf)
		if err != nil {
			if r.isRunning() {
				log.Printf("Relay read error: %v", err)
			}
			return
		}
		if n < 5 {
			continue
		}

		magic := binary.BigEndian.Uint32(buf[0:4])
		if magic != relayMagic {
			continue
		}

		msgType := buf[4]

		switch msgType {
		case relayTypeReg:
			vip, ok := r.parseRelayReg(buf[:n])
			if !ok || vip == "" {
				continue
			}
			r.mu.Lock()
			prevKey := ""
			if existing, ok := r.byAddr[addr.String()]; ok {
				prevKey = existing.VirtualIP
			}
			if prevKey != "" {
				delete(r.entries, prevKey)
			}
			if existing, ok := r.entries[vip]; ok {
				delete(r.byAddr, existing.UDPAddr.String())
			}
			r.entries[vip] = &RelayEntry{
				VirtualIP: vip,
				UDPAddr:   addr,
				lastSeen:  time.Now(),
			}
			r.byAddr[addr.String()] = r.entries[vip]
			r.mu.Unlock()
			log.Printf("Relay registered: %s at %s", vip, addr.String())

		case relayTypeDat:
			if n < 6 {
				continue
			}
			dstLen := int(buf[5])
			if dstLen < 1 || 5+1+dstLen > n {
				continue
			}
			destVIP := string(buf[6 : 6+dstLen])
			remaining := buf[6+dstLen : n]

			// Look up sender and destination under a single write lock
			// to avoid data race on lastSeen updates
			r.mu.Lock()
			sender, okSender := r.byAddr[addr.String()]
			if okSender {
				sender.lastSeen = time.Now()
			}
			entry, okDest := r.entries[destVIP]
			if okDest {
				entry.lastSeen = time.Now()
			}
			r.mu.Unlock()

			if !okSender {
				log.Printf("Relay: sender not found for addr %s, dest %s", addr.String(), destVIP)
				continue
			}
			if !okDest {
				log.Printf("Relay: dest %s not found, sender %s (%s)", destVIP, sender.VirtualIP, addr.String())
				continue
			}

			// remaining format: [inner_src_vip_len:1][inner_src_vip][actual_data]
			if len(remaining) < 2 {
				continue
			}
			innerSrcLen := int(remaining[0])
			if innerSrcLen < 1 || 1+innerSrcLen > len(remaining) {
				continue
			}
			actualData := remaining[1+innerSrcLen:]

			// Build forwarded packet: [src_vip_len:1][src_vip][actual_data]
			fwd := make([]byte, 1+len(sender.VirtualIP)+len(actualData))
			fwd[0] = byte(len(sender.VirtualIP))
			copy(fwd[1:], sender.VirtualIP)
			copy(fwd[1+len(sender.VirtualIP):], actualData)

			_, err = r.conn.WriteToUDP(fwd, entry.UDPAddr)
			if err != nil {
				log.Printf("Relay forward error to %s: %v", destVIP, err)
			}
		}
	}
}

func (r *Relay) cleanupLoop() {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-r.done:
			return
		case <-ticker.C:
			r.mu.Lock()
			now := time.Now()
			for vip, e := range r.entries {
				if now.Sub(e.lastSeen) > relayTimeout {
					delete(r.byAddr, e.UDPAddr.String())
					delete(r.entries, vip)
					log.Printf("Relay cleaned up stale entry: %s", vip)
				}
			}
			r.mu.Unlock()
		}
	}
}

func (r *Relay) isRunning() bool {
	select {
	case <-r.done:
		return false
	default:
		return true
	}
}
