package protocol

import (
	"context"
	"net"

	"github.com/maskedeken/gost-plugin/args"
	C "github.com/maskedeken/gost-plugin/constant"
	"github.com/maskedeken/gost-plugin/gost"
	"github.com/maskedeken/gost-plugin/registry"
	xtls "github.com/xtls/go"
)

var (
	xtlsSessionCache = xtls.NewLRUClientSessionCache(128)
)

type XTLSListener struct {
	*TCPListener
}

func (l *XTLSListener) AcceptConn() (net.Conn, error) {
	conn := <-l.connChan
	if xtlsConn, ok := conn.(*xtls.Conn); ok {
		xtlsConn.RPRX = true
		xtlsConn.DirectMode = true
		// xtlsConn.SHOW = xtls_show
		// xtlsConn.MARK = "XTLS"
	}
	return conn, nil
}

func NewXTLSListener(ctx context.Context) (gost.Listener, error) {
	inner, err := NewTCPListener(ctx)
	if err != nil {
		return nil, err
	}

	xtlsConfig, err := buildServerXTLSConfig(ctx)
	if err != nil {
		return nil, err
	}

	l := &XTLSListener{inner.(*TCPListener)}
	ln := l.listener
	l.listener = xtls.NewListener(ln, xtlsConfig) // turn listener into xtls.Listener
	return l, nil
}

type XTLSTransporter struct {
	*TCPTransporter
}

func (t *XTLSTransporter) DialConn() (net.Conn, error) {
	conn, err := t.TCPTransporter.DialConn()
	if err != nil {
		return nil, err
	}

	xtlsConn := xtls.Client(conn, buildClientXTLSConfig(t.ctx))
	err = xtlsConn.Handshake()
	if err != nil {
		return nil, err
	}

	xtlsConn.RPRX = true
	xtlsConn.DirectMode = true
	// xtlsConn.SHOW = xtls_show
	// xtlsConn.MARK = "XTLS"
	return xtlsConn, nil
}

func NewXTLSTransporter(ctx context.Context) (gost.Transporter, error) {
	inner, err := NewTCPTransporter(ctx)
	if err != nil {
		return nil, err
	}

	return &XTLSTransporter{inner.(*TCPTransporter)}, nil
}

func buildServerXTLSConfig(ctx context.Context) (*xtls.Config, error) {
	options := ctx.Value(C.OPTIONS).(*args.Options)
	cert, err := xtls.LoadX509KeyPair(options.Cert, options.Key)
	if err != nil {
		return nil, err
	}

	return &xtls.Config{Certificates: []xtls.Certificate{cert}}, nil
}

func buildClientXTLSConfig(ctx context.Context) *xtls.Config {
	options := ctx.Value(C.OPTIONS).(*args.Options)

	xtlsConfig := &xtls.Config{
		ClientSessionCache:     xtlsSessionCache,
		NextProtos:             []string{"http/1.1"},
		InsecureSkipVerify:     options.Insecure,
		SessionTicketsDisabled: true,
	}
	if options.ServerName != "" {
		xtlsConfig.ServerName = options.ServerName
	}

	return xtlsConfig
}

func init() {
	registry.RegisterListener("xtls", NewXTLSListener)
	registry.RegisterTransporter("xtls", NewXTLSTransporter)
}
