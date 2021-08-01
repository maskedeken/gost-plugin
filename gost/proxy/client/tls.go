package client

import (
	"context"
	"crypto/tls"
	"net"

	"github.com/maskedeken/gost-plugin/args"
	C "github.com/maskedeken/gost-plugin/constant"
	"github.com/maskedeken/gost-plugin/gost"
	"github.com/maskedeken/gost-plugin/gost/proxy"
	"github.com/maskedeken/gost-plugin/registry"
)

var (
	tlsSessionCache = tls.NewLRUClientSessionCache(128)
)

// TLSTransporter is Transporter which handles tls
type TLSTransporter struct {
	*proxy.TCPTransporter
}

// DialConn implements gost.Transporter.DialConn()
func (t *TLSTransporter) DialConn() (net.Conn, error) {
	conn, err := t.TCPTransporter.DialConn()
	if err != nil {
		return nil, err
	}

	tlsConn := tls.Client(conn, buildClientTLSConfig(t.Context))
	err = tlsConn.Handshake()
	if err != nil {
		return nil, err
	}

	return tlsConn, nil
}

// NewTLSTransporter is constructor for TLSTransporter
func NewTLSTransporter(ctx context.Context) (gost.Transporter, error) {
	inner, err := proxy.NewTCPTransporter(ctx)
	if err != nil {
		return nil, err
	}

	return &TLSTransporter{inner.(*proxy.TCPTransporter)}, nil
}

func buildClientTLSConfig(ctx context.Context) *tls.Config {
	options := ctx.Value(C.OPTIONS).(*args.Options)

	tlsConfig := &tls.Config{
		ClientSessionCache:     tlsSessionCache,
		InsecureSkipVerify:     options.Insecure,
		SessionTicketsDisabled: true,
	}

	tlsConfig.CurvePreferences = []tls.CurveID{29}
	tlsConfig.MinVersion = tls.VersionTLS13
	tlsConfig.CipherSuites = []uint16{
		tls.TLS_CHACHA20_POLY1305_SHA256,
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
	registry.RegisterTransporter("tls", NewTLSTransporter)
}
