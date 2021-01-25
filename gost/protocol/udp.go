package protocol

import (
	"context"
	"net"

	"github.com/maskedeken/gost-plugin/registry"
)

func ListenPacket(ctx context.Context, srcAddr net.Addr) (net.PacketConn, error) {
	if srcAddr == nil {
		srcAddr = &net.UDPAddr{IP: net.IPv4zero, Port: 0}
	}

	var lc net.ListenConfig
	lc.Control = registry.GetListenControl(ctx)
	return lc.ListenPacket(ctx, srcAddr.Network(), srcAddr.String())
}
