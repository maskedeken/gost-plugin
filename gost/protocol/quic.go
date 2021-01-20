package protocol

import (
	"context"
	"errors"
	"net"
	"sync"
	"time"

	"github.com/lucas-clemente/quic-go"
	"github.com/maskedeken/gost-plugin/args"
	C "github.com/maskedeken/gost-plugin/constant"
	"github.com/maskedeken/gost-plugin/gost"
	"github.com/maskedeken/gost-plugin/log"
	"github.com/maskedeken/gost-plugin/registry"
)

// QUICListener is Listener which handles quic protocol
type QUICListener struct {
	listener quic.Listener
	ctx      context.Context
	connChan chan net.Conn
}

func (l *QUICListener) acceptStreams(session quic.Session) {
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

		conn := &interConn{
			stream: stream,
			local:  session.LocalAddr(),
			remote: session.RemoteAddr(),
		}

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
	tlsConfig.NextProtos = []string{C.QUIC_ALPN}

	quicConfig := &quic.Config{
		ConnectionIDLength:    12,
		HandshakeTimeout:      time.Second * 8,
		MaxIdleTimeout:        time.Second * 45,
		MaxIncomingStreams:    32,
		MaxIncomingUniStreams: -1,
	}

	options := ctx.Value(C.OPTIONS).(*args.Options)
	ln, err := quic.ListenAddr(options.GetLocalAddr(), tlsConfig, quicConfig)
	if err != nil {
		return nil, err
	}

	return &QUICListener{
		listener: ln,
		ctx:      ctx,
		connChan: make(chan net.Conn, 1024),
	}, nil
}

// QUICTransporter is Transporter which handles quic protocol
type QUICTransporter struct {
	access  sync.Mutex
	ctx     context.Context
	session quic.Session
}

func (t *QUICTransporter) openStream() (net.Conn, error) {
	if !t.isSessionActive() {
		return nil, errors.New("session closed")
	}

	stream, err := t.session.OpenStream()
	if err != nil {
		return nil, err
	}

	return &interConn{
		stream: stream,
		local:  t.session.LocalAddr(),
		remote: t.session.RemoteAddr(),
	}, nil
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
	tlsConfig.NextProtos = []string{C.QUIC_ALPN}
	quicConfig := &quic.Config{
		ConnectionIDLength: 12,
		HandshakeTimeout:   time.Second * 8,
		MaxIdleTimeout:     time.Second * 30,
	}
	options := t.ctx.Value(C.OPTIONS).(*args.Options)
	session, err := quic.DialAddr(options.GetRemoteAddr(), tlsConfig, quicConfig)
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

type interConn struct {
	stream quic.Stream
	local  net.Addr
	remote net.Addr
}

func (c *interConn) Read(b []byte) (int, error) {
	return c.stream.Read(b)
}

func (c *interConn) Write(b []byte) (int, error) {
	return c.stream.Write(b)
}

func (c *interConn) Close() error {
	return c.stream.Close()
}

func (c *interConn) LocalAddr() net.Addr {
	return c.local
}

func (c *interConn) RemoteAddr() net.Addr {
	return c.remote
}

func (c *interConn) SetDeadline(t time.Time) error {
	return c.stream.SetDeadline(t)
}

func (c *interConn) SetReadDeadline(t time.Time) error {
	return c.stream.SetReadDeadline(t)
}

func (c *interConn) SetWriteDeadline(t time.Time) error {
	return c.stream.SetWriteDeadline(t)
}

func init() {
	registry.RegisterListener("quic", NewQUICListener)
	registry.RegisterTransporter("quic", NewQUICTransporter)
}
