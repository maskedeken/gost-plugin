package client

import (
	"context"
	"net"

	"github.com/maskedeken/gost-plugin/gost"
	"github.com/maskedeken/gost-plugin/mux"
	"github.com/maskedeken/gost-plugin/registry"
)

// MTLSTransporter is Transporter which handles multiplex tls
type MTLSTransporter struct {
	*TLSTransporter
	pool *mux.MuxPool
}

// DialConn implements gost.Transporter.DialConn()
func (t *MTLSTransporter) DialConn() (net.Conn, error) {
	return t.pool.DialMux(t.TLSTransporter.DialConn)
}

// NewMTLSTransporter is constructor for MTLSTransporter
func NewMTLSTransporter(ctx context.Context) (gost.Transporter, error) {
	inner, err := NewTLSTransporter(ctx)
	if err != nil {
		return nil, err
	}

	return &MTLSTransporter{
		TLSTransporter: inner.(*TLSTransporter),
		pool:           mux.NewMuxPool(ctx),
	}, nil
}

func init() {
	registry.RegisterTransporter("mtls", NewMTLSTransporter)
}
