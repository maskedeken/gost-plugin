package server

import (
	"context"
	"net"
	"syscall"

	"github.com/maskedeken/gost-plugin/args"
	C "github.com/maskedeken/gost-plugin/constant"
	"github.com/maskedeken/gost-plugin/gost"
	"github.com/maskedeken/gost-plugin/gost/proxy"
	"github.com/maskedeken/gost-plugin/gost/readv"
	"github.com/maskedeken/gost-plugin/registry"
	xtls "github.com/xtls/go"
)

// XTLSListener is Listener which handles XTLS
type XTLSListener struct {
	*proxy.TCPListener
	xtlsShow bool
}

// AcceptConn implements gost.Listener.AcceptConn()
func (l *XTLSListener) AcceptConn() (net.Conn, error) {
	conn := <-l.ConnChan
	if xConn, ok := conn.(*xtls.Conn); ok {
		var rawConn syscall.RawConn
		if sc, ok := xConn.NetConn().(syscall.Conn); ok {
			rawConn, _ = sc.SyscallConn()
		}

		xConn.RPRX = true
		xConn.DirectMode = true
		xConn.SHOW = l.xtlsShow
		xConn.MARK = "XTLS"
		conn = readv.NewReadVConn(xConn, rawConn)
	}
	return conn, nil
}

// NewXTLSListener is the constructor for XTLSListener
func NewXTLSListener(ctx context.Context) (gost.Listener, error) {
	inner, err := proxy.NewTCPListener(ctx)
	if err != nil {
		return nil, err
	}

	options := ctx.Value(C.OPTIONS).(*args.Options)
	xtlsConfig, err := buildServerXTLSConfig(ctx)
	if err != nil {
		return nil, err
	}

	l := &XTLSListener{
		TCPListener: inner.(*proxy.TCPListener),
		xtlsShow:    options.XTLSShow,
	}
	l.Listener = xtls.NewListener(l.Listener, xtlsConfig) // turn listener into xtls.Listener
	return l, nil
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

func init() {
	registry.RegisterListener("xtls", NewXTLSListener)
}
