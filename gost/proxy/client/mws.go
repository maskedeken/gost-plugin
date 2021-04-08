package client

import (
	"context"
	"net"

	"github.com/maskedeken/gost-plugin/gost"
	"github.com/maskedeken/gost-plugin/mux"
	"github.com/maskedeken/gost-plugin/registry"
)

// MWSTransporter is Transporter which handles multiplex websocket protocol
type MWSTransporter struct {
	*WSTransporter
	pool *mux.MuxPool
}

// DialConn implements gost.Transporter.DialConn()
func (t *MWSTransporter) DialConn() (net.Conn, error) {
	return t.pool.DialMux(t.WSTransporter.DialConn)
}

// NewMWSTransporter is constructor for MWSTransporter
func NewMWSTransporter(ctx context.Context) (gost.Transporter, error) {
	inner, err := NewWSTransporter(ctx)
	if err != nil {
		return nil, err
	}

	return &MWSTransporter{WSTransporter: inner.(*WSTransporter),
		pool: mux.NewMuxPool(ctx),
	}, nil
}

// MWSSTransporter is Transporter which handles multiplex websocket over tls
type MWSSTransporter struct {
	*WSSTransporter
	pool *mux.MuxPool
}

// DialConn implements gost.Transporter.DialConn()
func (t *MWSSTransporter) DialConn() (net.Conn, error) {
	return t.pool.DialMux(t.WSSTransporter.DialConn)
}

// NewMWSSTransporter is constructor for MWSSTransporter
func NewMWSSTransporter(ctx context.Context) (gost.Transporter, error) {
	inner, err := NewWSSTransporter(ctx)
	if err != nil {
		return nil, err
	}

	return &MWSSTransporter{WSSTransporter: inner.(*WSSTransporter),
		pool: mux.NewMuxPool(ctx),
	}, nil
}

func init() {
	registry.RegisterTransporter("mws", NewMWSTransporter)
	registry.RegisterTransporter("mwss", NewMWSSTransporter)
}
