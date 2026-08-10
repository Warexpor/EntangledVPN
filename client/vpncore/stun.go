package vpncore

import (
	"bytes"
	"crypto/rand"
	"encoding/binary"
	"fmt"
	"io"
	"net"
	"time"
)

const (
	stunMagicCookie   = 0x2112A442
	stunBindingReq    = 0x0001
	attrXORMappedAddr = 0x0020
	attrMappedAddr    = 0x0001
)

// Default STUN servers. Cloudflare first — some networks drop Google STUN (19302).
var defaultSTUNServers = []string{
	"stun.cloudflare.com:3478",
	"stun.l.google.com:19302",
}

// DiscoverPublicAddrFallback tries configured (if any) then defaultSTUNServers.
func DiscoverPublicAddrFallback(configured string, timeout time.Duration, conn *net.UDPConn) (string, string, error) {
	servers := make([]string, 0, 1+len(defaultSTUNServers))
	if configured != "" {
		servers = append(servers, configured)
	}
	servers = append(servers, defaultSTUNServers...)
	seen := map[string]bool{}
	per := timeout
	if n := len(servers); n > 1 {
		per = timeout / time.Duration(n)
		if per < 2*time.Second {
			per = 2 * time.Second
		}
	}
	var last error
	for _, s := range servers {
		if seen[s] {
			continue
		}
		seen[s] = true
		ea, err := DiscoverPublicAddr(s, per, conn)
		if err == nil {
			return ea, s, nil
		}
		last = err
	}
	if last == nil {
		last = fmt.Errorf("no stun servers")
	}
	return "", "", last
}

// DiscoverPublicAddr runs a STUN binding request on conn so the mapped port
// matches the UDP listen port used for P2P/relay.
func DiscoverPublicAddr(server string, timeout time.Duration, conn *net.UDPConn) (string, error) {
	if conn == nil {
		return "", fmt.Errorf("nil udp conn")
	}
	raddr, err := net.ResolveUDPAddr("udp4", server)
	if err != nil {
		return "", err
	}

	tid := make([]byte, 12)
	if _, err := io.ReadFull(rand.Reader, tid); err != nil {
		return "", fmt.Errorf("failed to generate transaction id: %v", err)
	}
	req := make([]byte, 20)
	binary.BigEndian.PutUint16(req[0:2], stunBindingReq)
	binary.BigEndian.PutUint16(req[2:4], 0)
	binary.BigEndian.PutUint32(req[4:8], stunMagicCookie)
	copy(req[8:20], tid)

	deadline := time.Now().Add(timeout)
	_ = conn.SetReadDeadline(deadline)
	defer conn.SetReadDeadline(time.Time{})

	if _, err := conn.WriteToUDP(req, raddr); err != nil {
		return "", err
	}

	buf := make([]byte, 1500)
	for {
		n, _, err := conn.ReadFromUDP(buf)
		if err != nil {
			return "", err
		}
		if n < 20 {
			continue
		}
		cookie := binary.BigEndian.Uint32(buf[4:8])
		if cookie != stunMagicCookie {
			continue
		}
		if !bytes.Equal(buf[8:20], tid) {
			continue
		}
		if addr, ok := parseSTUNMapped(buf[:n]); ok {
			return addr, nil
		}
	}
}

func parseSTUNMapped(buf []byte) (string, bool) {
	attrs := buf[20:]
	for len(attrs) >= 4 {
		attrType := binary.BigEndian.Uint16(attrs[0:2])
		attrLen := binary.BigEndian.Uint16(attrs[2:4])
		if int(4+attrLen) > len(attrs) {
			break
		}
		val := attrs[4 : 4+attrLen]
		if attrType == attrXORMappedAddr && len(val) >= 8 {
			family := val[1]
			if family == 0x01 {
				xorPort := binary.BigEndian.Uint16(val[2:4])
				port := int(xorPort ^ uint16(stunMagicCookie>>16))
				ip := make(net.IP, 4)
				for i := 0; i < 4; i++ {
					ip[i] = val[4+i] ^ byte((uint32(stunMagicCookie)>>(8*(3-i)))&0xFF)
				}
				return net.JoinHostPort(ip.String(), fmt.Sprintf("%d", port)), true
			}
		}
		if attrType == attrMappedAddr && len(val) >= 8 {
			family := val[1]
			if family == 0x01 {
				port := int(binary.BigEndian.Uint16(val[2:4]))
				ip := net.IP(val[4:8])
				return net.JoinHostPort(ip.String(), fmt.Sprintf("%d", port)), true
			}
		}
		pad := (4 - (int(attrLen) % 4)) % 4
		attrs = attrs[4+int(attrLen)+pad:]
	}
	return "", false
}
