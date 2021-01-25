package protocol

import (
	"context"
	"net"
	"time"

	"github.com/maskedeken/gost-plugin/args"
	C "github.com/maskedeken/gost-plugin/constant"
	"github.com/maskedeken/gost-plugin/errors"
	"github.com/maskedeken/gost-plugin/gost"
	"github.com/maskedeken/gost-plugin/log"
	"github.com/maskedeken/gost-plugin/registry"
	"github.com/xtaci/smux"
)

type tcpKeepAliveListener struct {
	*net.TCPListener
}

func (ln tcpKeepAliveListener) Accept() (c net.Conn, err error) {
	tc, err := ln.AcceptTCP()
	if err != nil {
		return
	}
	tc.SetKeepAlive(true)
	tc.SetKeepAlivePeriod(C.KEEP_ALIVE_TIME)
	return tc, nil
}

// TCPListener is Listener which handles tcp
type TCPListener struct {
	listener net.Listener
	connChan chan net.Conn
}

// Close implements gost.Listener.Close()
func (l *TCPListener) Close() error {
	return l.listener.Close()
}

// AcceptConn implements gost.Listener.AcceptConn()
func (l *TCPListener) AcceptConn() (conn net.Conn, err error) {
	conn = <-l.connChan
	return
}

// Serve implements gost.Listener.Serve()
func (l *TCPListener) Serve(ctx context.Context) error {
	keepAccepting(ctx, l.listener, l.connChan)
	return nil
}

// NewTCPListener is constructor for TCPListener
func NewTCPListener(ctx context.Context) (gost.Listener, error) {
	options := ctx.Value(C.OPTIONS).(*args.Options)
	lAddrStr := options.GetLocalAddr()
	lAddr, _ := net.ResolveTCPAddr("tcp", lAddrStr)
	ln, err := Listen(ctx, lAddr)
	if err != nil {
		return nil, err
	}

	l := &TCPListener{
		listener: &tcpKeepAliveListener{ln.(*net.TCPListener)},
		connChan: make(chan net.Conn, 1024),
	}

	return l, nil
}

// TCPTransporter is Listener which handles tcp
type TCPTransporter struct {
	ctx context.Context
}

// DialConn implements gost.Transporter.DialConn()
func (t *TCPTransporter) DialConn() (net.Conn, error) {
	options := t.ctx.Value(C.OPTIONS).(*args.Options)
	dialer := &net.Dialer{
		Timeout:   time.Second * 16,
		DualStack: true,
		LocalAddr: nil,
	}
	dialer.Control = registry.GetDialControl(t.ctx)

	return dialer.Dial("tcp", options.GetRemoteAddr())
}

// NewTCPTransporter is constructor for TCPTransporter
func NewTCPTransporter(ctx context.Context) (gost.Transporter, error) {
	return &TCPTransporter{ctx}, nil
}

func init() {
	registry.RegisterListener("tcp", NewTCPListener)
	registry.RegisterTransporter("tcp", NewTCPTransporter)
}

func Listen(ctx context.Context, srcAddr net.Addr) (net.Listener, error) {
	if srcAddr == nil {
		srcAddr = &net.TCPAddr{IP: net.IPv4zero, Port: 0}
	}

	var lc net.ListenConfig
	lc.Control = registry.GetListenControl(ctx)
	return lc.Listen(ctx, srcAddr.Network(), srcAddr.String())
}

func keepAccepting(ctx context.Context, listener net.Listener, connChan chan net.Conn) {
	for {
		conn, err := listener.Accept()
		if err != nil {
			if errors.IsClosed(err) {
				break
			}

			log.Errorf("failed to accept connection: %s", err)

			select {
			case <-ctx.Done():
				log.Debugln("context ends")
				return
			default:
				continue
			}
		}

		select {
		case connChan <- conn:
		default:
			log.Warnln("connection queue is full")
			conn.Close()
		}
	}
}

func keepMuxAccepting(ctx context.Context, listener net.Listener, connChan chan net.Conn) {
	for {
		conn, err := listener.Accept()
		if err != nil {
			if errors.IsClosed(err) {
				break
			}

			log.Errorf("failed to accept connection: %s", err)

			select {
			case <-ctx.Done():
				log.Debugln("context ends")
				return
			default:
				continue
			}
		}

		go serveMux(conn, connChan)
	}
}

func serveMux(conn net.Conn, connChan chan net.Conn) {
	smuxConfig := smux.DefaultConfig()
	session, err := smux.Server(conn, smuxConfig)
	if err != nil {
		if !errors.IsEOF(err) && !errors.IsClosed(err) {
			log.Errorf("failed to accept smux session: %s", err)
		}

		return
	}

	defer session.Close()

	var stream *smux.Stream
	for {
		stream, err = session.AcceptStream()
		if err != nil {
			if !errors.IsEOF(err) && !errors.IsClosed(err) {
				log.Errorf("failed to accept smux stream: %s", err)
			}

			return
		}

		select {
		case connChan <- stream:
		default:
			log.Warnln("connection queue is full")
			stream.Close()
		}
	}
}
