package protocol

import (
	"context"
	"crypto/tls"
	"net"

	"github.com/maskedeken/gost-plugin/args"
	C "github.com/maskedeken/gost-plugin/constant"
	"github.com/maskedeken/gost-plugin/gost"
	"github.com/maskedeken/gost-plugin/mux"
	"github.com/maskedeken/gost-plugin/registry"
)

var (
	tlsSessionCache = tls.NewLRUClientSessionCache(128)
)

type TLSListener struct {
	*TCPListener
}

func NewTLSListener(ctx context.Context) (gost.Listener, error) {
	inner, err := NewTCPListener(ctx)
	if err != nil {
		return nil, err
	}

	tlsConfig, err := buildServerTLSConfig(ctx)
	if err != nil {
		return nil, err
	}

	l := &TLSListener{inner.(*TCPListener)}
	ln := l.listener
	l.listener = tls.NewListener(ln, tlsConfig)
	return l, nil
}

type MTLSListener struct {
	*TLSListener
}

func (l *MTLSListener) Serve(ctx context.Context) error {
	keepMuxAccepting(ctx, l.listener, l.connChan)
	return nil
}

func NewMTLSListener(ctx context.Context) (gost.Listener, error) {
	inner, err := NewTLSListener(ctx)
	if err != nil {
		return nil, err
	}

	return &MTLSListener{inner.(*TLSListener)}, nil
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

func NewTLSTransporter(ctx context.Context) (gost.Transporter, error) {
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

func buildServerTLSConfig(ctx context.Context) (*tls.Config, error) {
	options := ctx.Value(C.OPTIONS).(*args.Options)
	cert, err := tls.LoadX509KeyPair(options.Cert, options.Key)
	if err != nil {
		return nil, err
	}

	return &tls.Config{Certificates: []tls.Certificate{cert}}, nil
}

func buildClientTLSConfig(ctx context.Context) *tls.Config {
	options := ctx.Value(C.OPTIONS).(*args.Options)

	tlsConfig := &tls.Config{
		ClientSessionCache:     tlsSessionCache,
		NextProtos:             []string{"http/1.1"},
		InsecureSkipVerify:     options.Insecure,
		SessionTicketsDisabled: true,
	}
	if options.ServerName != "" {
		tlsConfig.ServerName = options.ServerName
	}

	return tlsConfig
}

func init() {
	registry.RegisterListener("tls", NewTLSListener)
	registry.RegisterTransporter("tls", NewTLSTransporter)

	registry.RegisterListener("mtls", NewMTLSListener)
	registry.RegisterTransporter("mtls", NewMTLSTransporter)
}
