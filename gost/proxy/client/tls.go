package client

import (
	"context"
	"crypto/tls"
	"net"
	"strings"

	"github.com/maskedeken/gost-plugin/args"
	C "github.com/maskedeken/gost-plugin/constant"
	"github.com/maskedeken/gost-plugin/gost"
	"github.com/maskedeken/gost-plugin/gost/proxy"
	"github.com/maskedeken/gost-plugin/registry"

	"github.com/maskedeken/gost-plugin/log"
	utls "github.com/refraction-networking/utls"
)

var (
	tlsSessionCache = tls.NewLRUClientSessionCache(128)
)

type tlsConn interface {
	net.Conn
	Handshake() error
	NegotiatedProtocol() string
}

type gotlsConnWrapper struct {
	*tls.Conn
}

func (c *gotlsConnWrapper) NegotiatedProtocol() string {
	return c.ConnectionState().NegotiatedProtocol
}

type utlsConnWrapper struct {
	*utls.UConn
}

func (c *utlsConnWrapper) NegotiatedProtocol() string {
	return c.ConnectionState().NegotiatedProtocol
}

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

	options := t.Context.Value(C.OPTIONS).(*args.Options)
	tlsConfig := buildClientTLSConfig(t.Context)
	tlsConn := newClientTLSConn(conn, tlsConfig, options.Fingerprint)
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

func newClientTLSConn(underlyConn net.Conn, tlsConfig *tls.Config, fingerprint string) tlsConn {
	if fingerprint == "" {
		return &gotlsConnWrapper{tls.Client(underlyConn, tlsConfig)}
	}

	// use utls
	var helloID utls.ClientHelloID = utls.HelloChrome_Auto
	switch strings.ToLower(fingerprint) {
	case "chrome":
		helloID = utls.HelloChrome_Auto
	case "ios":
		helloID = utls.HelloIOS_Auto
	case "firefox":
		helloID = utls.HelloFirefox_Auto
	case "edge":
		helloID = utls.HelloEdge_Auto
	case "safari":
		helloID = utls.HelloSafari_Auto
	case "360browser":
		helloID = utls.Hello360_Auto
	case "qqbrowser":
		helloID = utls.HelloQQ_Auto
	default:
		log.Warnln("Fingerprint is invalid. Use Chrome by default.")
	}

	uConn := utls.UClient(underlyConn, &utls.Config{
		NextProtos:             tlsConfig.NextProtos,
		InsecureSkipVerify:     tlsConfig.InsecureSkipVerify,
		SessionTicketsDisabled: tlsConfig.SessionTicketsDisabled,
		ServerName:             tlsConfig.ServerName,
	}, helloID)
	return &utlsConnWrapper{uConn}
}

func init() {
	registry.RegisterTransporter("tls", NewTLSTransporter)
}
