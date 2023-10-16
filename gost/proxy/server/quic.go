package server

import (
	"context"
	"net"

	"github.com/maskedeken/gost-plugin/args"
	C "github.com/maskedeken/gost-plugin/constant"
	"github.com/maskedeken/gost-plugin/gost"
	Q "github.com/maskedeken/gost-plugin/gost/protocol/quic"
	"github.com/maskedeken/gost-plugin/gost/proxy"
	"github.com/maskedeken/gost-plugin/log"
	"github.com/maskedeken/gost-plugin/registry"
	"github.com/quic-go/quic-go"
)

// QUICListener is Listener which handles quic protocol
type QUICListener struct {
	listener *quic.Listener
	ctx      context.Context
	connChan chan net.Conn
}

func (l *QUICListener) acceptStreams(session quic.Connection) {
	defer session.CloseWithError(0, "")

	for {
		stream, err := session.AcceptStream(context.Background())
		if err != nil {
			log.Errorf("failed to accept QUIC stream: %s", err)

			select {
			case <-session.Context().Done():
				return
			case <-l.ctx.Done():
				log.Debugln("context ends")
				return
			default:
				// time.Sleep(time.Second)
				continue
			}
		}

		conn := Q.NewQUICConn(stream, session.LocalAddr(), session.RemoteAddr())

		select {
		case l.connChan <- conn:
		default:
			log.Warnln("connection queue is full")
			conn.Close()
		}
	}
}

// Serve implements gost.Listener.Serve()
func (l *QUICListener) Serve(ctx context.Context) error {
	for {
		conn, err := l.listener.Accept(context.Background())
		if err != nil {
			log.Errorf("failed to accept QUIC sessions: %s", err)

			select {
			case <-l.ctx.Done():
				log.Debugln("context ends")
				return nil
			default:
				// time.Sleep(time.Second)
				continue
			}
		}

		go l.acceptStreams(conn)
	}
}

// AcceptConn implements gost.Listener.AcceptConn()
func (l *QUICListener) AcceptConn() (net.Conn, error) {
	return <-l.connChan, nil
}

// AcceptConn implements gost.Listener.Close()
func (l *QUICListener) Close() error {
	return l.listener.Close()
}

// NewQUICListener is constructor for QUICListener
func NewQUICListener(ctx context.Context) (gost.Listener, error) {
	tlsConfig, err := buildServerTLSConfig(ctx)
	if err != nil {
		return nil, err
	}
	tlsConfig.NextProtos = []string{"h2", "http/1.1"}

	quicConfig := &quic.Config{
		KeepAlivePeriod:       0,
		MaxIncomingStreams:    32,
		MaxIncomingUniStreams: -1,
	}

	options := ctx.Value(C.OPTIONS).(*args.Options)
	lAddrStr := options.GetLocalAddr()
	lAddr, _ := net.ResolveUDPAddr("udp", lAddrStr)
	pConn, err := proxy.ListenPacket(ctx, lAddr)
	if err != nil {
		return nil, err
	}

	ln, err := quic.Listen(pConn, tlsConfig, quicConfig)
	if err != nil {
		return nil, err
	}

	return &QUICListener{
		listener: ln,
		ctx:      ctx,
		connChan: make(chan net.Conn, 1024),
	}, nil
}

func init() {
	registry.RegisterListener("quic", NewQUICListener)
}
