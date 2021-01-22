package protocol

import (
	"context"
	"crypto/tls"
	"errors"
	"net"

	"github.com/maskedeken/gost-plugin/args"
	C "github.com/maskedeken/gost-plugin/constant"
	"github.com/maskedeken/gost-plugin/gost"
	"github.com/maskedeken/gost-plugin/registry"
)

var (
	tlsSessionCache = tls.NewLRUClientSessionCache(128)

	errNoCertSpecified = errors.New("No TLS cert specified")
)

// TLSListener is Listener which handles tls
type TLSListener struct {
	*TCPListener
}

// NewTLSListener is constructor for TLSListener
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

// TLSTransporter is Transporter which handles tls
type TLSTransporter struct {
	*TCPTransporter
}

// DialConn implements gost.Transporter.DialConn()
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

// NewTLSTransporter is constructor for TLSTransporter
func NewTLSTransporter(ctx context.Context) (gost.Transporter, error) {
	inner, err := NewTCPTransporter(ctx)
	if err != nil {
		return nil, err
	}

	return &TLSTransporter{inner.(*TCPTransporter)}, nil
}

func buildServerTLSConfig(ctx context.Context) (*tls.Config, error) {
	options := ctx.Value(C.OPTIONS).(*args.Options)
	if options.Cert == "" || options.Key == "" {
		return nil, errNoCertSpecified
	}

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
	} else {
		ip := net.ParseIP(options.RemoteAddr)
		if ip == nil {
			// if remoteAddr is domain
			tlsConfig.ServerName = options.RemoteAddr
		}
	}

	return tlsConfig
}

func init() {
	registry.RegisterListener("tls", NewTLSListener)
	registry.RegisterTransporter("tls", NewTLSTransporter)
}
