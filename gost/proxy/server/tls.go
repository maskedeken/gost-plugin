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

	return &tls.Config{Certificates: []tls.Certificate{cert}}, nil
}

func init() {
	registry.RegisterListener("tls", NewTLSListener)
}
