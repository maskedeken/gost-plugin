package server

import (
	"context"

	"github.com/maskedeken/gost-plugin/gost"
	"github.com/maskedeken/gost-plugin/gost/proxy"
	"github.com/maskedeken/gost-plugin/registry"
)

// MTLSListener is Listener which handles multiplex tls
type MTLSListener struct {
	*TLSListener
}

// Serve implements gost.Listener.Serve()
func (l *MTLSListener) Serve(ctx context.Context) error {
	proxy.KeepMuxAccepting(ctx, l.Listener, l.ConnChan)
	return nil
}

// NewMTLSListener is constructor for MTLSListener
func NewMTLSListener(ctx context.Context) (gost.Listener, error) {
	inner, err := NewTLSListener(ctx)
	if err != nil {
		return nil, err
	}

	return &MTLSListener{inner.(*TLSListener)}, nil
}

func init() {
	registry.RegisterListener("mtls", NewMTLSListener)
}
