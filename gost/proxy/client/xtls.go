package client

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

var (
	xtlsSessionCache = xtls.NewLRUClientSessionCache(128)
)

// XTLSTransporter is Transporter which handles XTLS
type XTLSTransporter struct {
	*proxy.TCPTransporter
	xtlsShow bool
}

// DialConn implements gost.Transporter.DialConn()
func (t *XTLSTransporter) DialConn() (net.Conn, error) {
	conn, err := t.TCPTransporter.DialConn()
	if err != nil {
		return nil, err
	}

	xConn := xtls.Client(conn, buildClientXTLSConfig(t.Context))
	err = xConn.Handshake()
	if err != nil {
		return nil, err
	}

	var rawConn syscall.RawConn
	if sc, ok := xConn.NetConn().(syscall.Conn); ok {
		rawConn, _ = sc.SyscallConn()
	}

	xConn.RPRX = true
	xConn.DirectMode = true
	xConn.SHOW = t.xtlsShow
	xConn.MARK = "XTLS"
	return readv.NewReadVConn(xConn, rawConn), nil
}

// NewXTLSTransporter is the constructor for XTLSTransporter
func NewXTLSTransporter(ctx context.Context) (gost.Transporter, error) {
	inner, err := proxy.NewTCPTransporter(ctx)
	if err != nil {
		return nil, err
	}

	options := ctx.Value(C.OPTIONS).(*args.Options)
	return &XTLSTransporter{
		TCPTransporter: inner.(*proxy.TCPTransporter),
		xtlsShow:       options.XTLSShow,
	}, nil
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
	registry.RegisterTransporter("xtls", NewXTLSTransporter)
}
