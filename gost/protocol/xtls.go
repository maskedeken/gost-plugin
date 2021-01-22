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

// XTLSListener is Listener which handles XTLS
type XTLSListener struct {
	*TCPListener
	xtlsShow bool
}

// AcceptConn implements gost.Listener.AcceptConn()
func (l *XTLSListener) AcceptConn() (net.Conn, error) {
	conn := <-l.connChan
	if xtlsConn, ok := conn.(*xtls.Conn); ok {
		xtlsConn.RPRX = true
		xtlsConn.DirectMode = true
		xtlsConn.SHOW = l.xtlsShow
		xtlsConn.MARK = "XTLS"
	}
	return conn, nil
}

// NewXTLSListener is the constructor for XTLSListener
func NewXTLSListener(ctx context.Context) (gost.Listener, error) {
	inner, err := NewTCPListener(ctx)
	if err != nil {
		return nil, err
	}

	options := ctx.Value(C.OPTIONS).(*args.Options)
	xtlsConfig, err := buildServerXTLSConfig(ctx)
	if err != nil {
		return nil, err
	}

	l := &XTLSListener{
		TCPListener: inner.(*TCPListener),
		xtlsShow:    options.XTLSShow,
	}
	ln := l.listener
	l.listener = xtls.NewListener(ln, xtlsConfig) // turn listener into xtls.Listener
	return l, nil
}

// XTLSTransporter is Transporter which handles XTLS
type XTLSTransporter struct {
	*TCPTransporter
	xtlsShow bool
}

// DialConn implements gost.Transporter.DialConn()
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
	xtlsConn.SHOW = t.xtlsShow
	xtlsConn.MARK = "XTLS"
	return xtlsConn, nil
}

// NewXTLSTransporter is the constructor for XTLSTransporter
func NewXTLSTransporter(ctx context.Context) (gost.Transporter, error) {
	inner, err := NewTCPTransporter(ctx)
	if err != nil {
		return nil, err
	}

	options := ctx.Value(C.OPTIONS).(*args.Options)
	return &XTLSTransporter{
		TCPTransporter: inner.(*TCPTransporter),
		xtlsShow:       options.XTLSShow,
	}, nil
}

func buildServerXTLSConfig(ctx context.Context) (*xtls.Config, error) {
	options := ctx.Value(C.OPTIONS).(*args.Options)
	if options.Cert == "" || options.Key == "" {
		return nil, errNoCertSpecified
	}

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
	} else {
		if net.ParseIP(options.RemoteAddr) == nil {
			// if remoteAddr is domain
			xtlsConfig.ServerName = options.RemoteAddr
		}
	}

	return xtlsConfig
}

func init() {
	registry.RegisterListener("xtls", NewXTLSListener)
	registry.RegisterTransporter("xtls", NewXTLSTransporter)
}
