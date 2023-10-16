package client

import (
	"context"
	"errors"
	"net"
	"sync"

	"github.com/maskedeken/gost-plugin/args"
	C "github.com/maskedeken/gost-plugin/constant"
	"github.com/maskedeken/gost-plugin/gost"
	Q "github.com/maskedeken/gost-plugin/gost/protocol/quic"
	"github.com/maskedeken/gost-plugin/gost/proxy"
	"github.com/maskedeken/gost-plugin/log"
	"github.com/maskedeken/gost-plugin/registry"
	"github.com/quic-go/quic-go"
)

// QUICTransporter is Transporter which handles quic protocol
type QUICTransporter struct {
	access  sync.Mutex
	ctx     context.Context
	session quic.Connection
}

func (t *QUICTransporter) openStream() (net.Conn, error) {
	if !t.isSessionActive() {
		return nil, errors.New("session closed")
	}

	stream, err := t.session.OpenStream()
	if err != nil {
		return nil, err
	}

	return Q.NewQUICConn(stream, t.session.LocalAddr(), t.session.RemoteAddr()), nil
}

func (t *QUICTransporter) DialConn() (net.Conn, error) {
	t.access.Lock()
	defer t.access.Unlock()

	if t.isSessionActive() {
		return t.openStream()
	}

	if t.session != nil {
		if err := t.session.CloseWithError(0, ""); err != nil {
			log.Errorf("failed close QUIC session: %S", err)
		}
	}

	tlsConfig := buildClientTLSConfig(t.ctx)
	tlsConfig.NextProtos = []string{"h2", "http/1.1"}
	quicConfig := &quic.Config{
		KeepAlivePeriod: 0,
	}
	options := t.ctx.Value(C.OPTIONS).(*args.Options)
	udpAddrStr := options.GetRemoteAddr()
	udpAddr, _ := net.ResolveUDPAddr("udp", udpAddrStr)
	pConn, err := proxy.ListenPacket(t.ctx, nil)
	if err != nil {
		return nil, err
	}

	session, err := quic.Dial(context.Background(), pConn, udpAddr, tlsConfig, quicConfig)
	if err != nil {
		return nil, err
	}
	t.session = session

	return t.openStream()
}

func (t *QUICTransporter) isSessionActive() bool {
	if t.session == nil {
		return false
	}

	select {
	case <-t.session.Context().Done():
		return false
	default:
		return true
	}
}

func NewQUICTransporter(ctx context.Context) (gost.Transporter, error) {
	return &QUICTransporter{
		ctx: ctx,
	}, nil
}

func init() {
	registry.RegisterTransporter("quic", NewQUICTransporter)
}
