package net

import (
	"context"
	"crypto/tls"
	"net"

	"github.com/maskedeken/gost-plugin/args"
	C "github.com/maskedeken/gost-plugin/constant"
	"github.com/maskedeken/gost-plugin/mux"
)

var (
	globalSessionCache = tls.NewLRUClientSessionCache(128)
)

type Transporter interface {
	DialConn() (net.Conn, error)
}

type TCPTransporter struct {
	ctx context.Context
}

func (t *TCPTransporter) DialConn() (net.Conn, error) {
	options := t.ctx.Value(C.OPTIONS).(*args.Options)
	return net.Dial("tcp", options.GetRemoteAddr())
}

func NewTCPTransporter(ctx context.Context) (Transporter, error) {
	return &TCPTransporter{ctx}, nil
}

type TLSTransporter struct {
	*TCPTransporter
}

func (t *TLSTransporter) DialConn() (net.Conn, error) {
	conn, err := t.TCPTransporter.DialConn()
	if err != nil {
		return nil, err
	}

	tlsConn := tls.Client(conn, buildClientTLSConfig(t.ctx))
	err = tlsConn.Handshake()
	if err != nil {
		return nil, err
	}

	return tlsConn, nil
}

func NewTLSTransporter(ctx context.Context) (Transporter, error) {
	inner, err := NewTCPTransporter(ctx)
	if err != nil {
		return nil, err
	}

	return &TLSTransporter{inner.(*TCPTransporter)}, nil
}

type MTLSTransporter struct {
	*TLSTransporter
	pool *mux.MuxPool
}

func (t *MTLSTransporter) DialConn() (net.Conn, error) {
	return t.pool.DialMux(t.TLSTransporter.DialConn)
}

func NewMTLSTransporter(ctx context.Context) (Transporter, error) {
	inner, err := NewTLSTransporter(ctx)
	if err != nil {
		return nil, err
	}

	return &MTLSTransporter{
		TLSTransporter: inner.(*TLSTransporter),
		pool:           mux.NewMuxPool(ctx),
	}, nil
}

type WSTransporter struct {
	*TCPTransporter
}

func (t *WSTransporter) DialConn() (net.Conn, error) {
	conn, err := t.TCPTransporter.DialConn()
	if err != nil {
		return nil, err
	}

	wsConn, err := WebsocketClientConn(t.ctx, conn)
	if err != nil {
		return nil, err
	}

	return wsConn, nil
}

func NewWSTransporter(ctx context.Context) (Transporter, error) {
	inner, err := NewTCPTransporter(ctx)
	if err != nil {
		return nil, err
	}

	return &WSTransporter{inner.(*TCPTransporter)}, nil
}

type WSSTransporter struct {
	*TLSTransporter
}

func (t *WSSTransporter) DialConn() (net.Conn, error) {
	conn, err := t.TLSTransporter.DialConn()
	if err != nil {
		return nil, err
	}

	wsConn, err := WebsocketClientConn(t.ctx, conn)
	if err != nil {
		return nil, err
	}

	return wsConn, nil
}

func NewWSSTransporter(ctx context.Context) (Transporter, error) {
	inner, err := NewTLSTransporter(ctx)
	if err != nil {
		return nil, err
	}

	return &WSSTransporter{inner.(*TLSTransporter)}, nil
}

type MWSTransporter struct {
	*WSTransporter
	pool *mux.MuxPool
}

func (t *MWSTransporter) DialConn() (net.Conn, error) {
	return t.pool.DialMux(t.WSTransporter.DialConn)
}

func NewMWSTransporter(ctx context.Context) (Transporter, error) {
	inner, err := NewWSTransporter(ctx)
	if err != nil {
		return nil, err
	}

	return &MWSTransporter{WSTransporter: inner.(*WSTransporter),
		pool: mux.NewMuxPool(ctx),
	}, nil
}

type MWSSTransporter struct {
	*WSSTransporter
	pool *mux.MuxPool
}

func (t *MWSSTransporter) DialConn() (net.Conn, error) {
	return t.pool.DialMux(t.WSSTransporter.DialConn)
}

func NewMWSSTransporter(ctx context.Context) (Transporter, error) {
	inner, err := NewWSSTransporter(ctx)
	if err != nil {
		return nil, err
	}

	return &MWSSTransporter{WSSTransporter: inner.(*WSSTransporter),
		pool: mux.NewMuxPool(ctx),
	}, nil
}

func buildClientTLSConfig(ctx context.Context) *tls.Config {
	options := ctx.Value(C.OPTIONS).(*args.Options)

	tlsConfig := &tls.Config{
		ClientSessionCache:     globalSessionCache,
		NextProtos:             []string{"http/1.1"},
		InsecureSkipVerify:     options.Insecure,
		SessionTicketsDisabled: true,
	}
	if options.Hostname != "" {
		tlsConfig.ServerName = options.Hostname
	}

	return tlsConfig
}
