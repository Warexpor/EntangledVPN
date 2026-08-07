package vpncore

import (
	"net"
)

type PacketRouter struct {
	peerManager *PeerManager
	localIP     string
	broadcastIP string
	OnSendToTUN func([]byte)
	OnAddRoute  func(string)
	OnLog       func(string, ...interface{})
}

func (r *PacketRouter) logf(format string, args ...interface{}) {
	if r.OnLog != nil {
		r.OnLog(format, args...)
	}
}

func NewPacketRouter(localIP string, peerManager *PeerManager) *PacketRouter {
	local := net.ParseIP(localIP).To4()
	bcast := net.IPv4(local[0], local[1], local[2], 255)
	return &PacketRouter{
		localIP:     localIP,
		peerManager: peerManager,
		broadcastIP: bcast.String(),
	}
}

func (r *PacketRouter) HandleTUNPacket(data []byte) {
	if len(data) < 20 {
		return
	}
	dstIP := net.IP(data[16:20])
	dstStr := dstIP.String()

	if dstIP.IsMulticast() || dstIP.IsLinkLocalUnicast() || dstIP.Equal(net.IPv4bcast) {
		return
	}
	if dstStr == "255.255.255.255" {
		return
	}
	if dstIP[0] == 0 {
		return
	}

	if dstStr == r.broadcastIP {
		r.peerManager.BroadcastToAll(data, "")
		return
	}
	if peer := r.peerManager.GetPeerByIP(dstStr); peer != nil {
		r.peerManager.SendToPeer(peer.ID, data)
	}
}

func (r *PacketRouter) HandlePeerPacket(fromID string, data []byte) {
	if len(data) < 20 {
		return
	}
	if data[0]>>4 != 4 {
		return
	}
	dst := net.IP(data[16:20]).String()
	if dst == r.broadcastIP {
		if r.OnSendToTUN != nil {
			r.OnSendToTUN(data)
		}
		return
	}
	if dst != r.localIP {
		return
	}
	if r.OnSendToTUN != nil {
		r.OnSendToTUN(data)
	}
}

func (r *PacketRouter) AddRoute(peerIP string) {
	r.logf("Adding route for %s", peerIP)
	if r.OnAddRoute != nil {
		r.OnAddRoute(peerIP)
	}
}
