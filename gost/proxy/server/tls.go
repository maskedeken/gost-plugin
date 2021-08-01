package server

import (
	"context"
	"crypto/tls"
	"errors"

	"github.com/maskedeken/gost-plugin/args"
	C "github.com/maskedeken/gost-plugin/constant"
	"github.com/maskedeken/gost-plugin/gost"
	"github.com/maskedeken/gost-plugin/gost/proxy"
	"github.com/maskedeken/gost-plugin/registry"
)

var (
	errNoCertSpecified = errors.New("No TLS cert specified")
)

// TLSListener is Listener which handles tls
type TLSListener struct {
	*proxy.TCPListener
}

// NewTLSListener is constructor for TLSListener
func NewTLSListener(ctx context.Context) (gost.Listener, error) {
	inner, err := proxy.NewTCPListener(ctx)
	if err != nil {
		return nil, err
	}

	tlsConfig, err := buildServerTLSConfig(ctx)
	if err != nil {
		return nil, err
	}

	l := &TLSListener{inner.(*proxy.TCPListener)}
	l.Listener = tls.NewListener(l.Listener, tlsConfig)
	return l, nil
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

        tlsConfig := &tls.Config{
        }

        tlsConfig.CurvePreferences = []tls.CurveID{29}
        tlsConfig.MinVersion = tls.VersionTLS13
        tlsConfig.CipherSuites = []uint16{
		tls.TLS_CHACHA20_POLY1305_SHA256,
		tls.TLS_ECDHE_ECDSA_WITH_CHACHA20_POLY1305,
		tls.TLS_ECDHE_ECDSA_WITH_CHACHA20_POLY1305_SHA256,
		tls.TLS_ECDHE_RSA_WITH_CHACHA20_POLY1305,
		tls.TLS_ECDHE_RSA_WITH_CHACHA20_POLY1305_SHA256,
	}
	tlsConfig.Certificates = []tls.Certificate{cert}

	return tlsConfig, nil
}

func init() {
	registry.RegisterListener("tls", NewTLSListener)
}
