package net

import (
	"context"
	"crypto/tls"
	"io"
	"net"
	"net/http"
	"time"

	"github.com/gorilla/websocket"
	"github.com/maskedeken/gost-plugin/args"
	C "github.com/maskedeken/gost-plugin/constant"
	"github.com/maskedeken/gost-plugin/errors"
	"github.com/maskedeken/gost-plugin/log"
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

type Listener interface {
	io.Closer
	Serve(context.Context) error
	AcceptConn() (net.Conn, error)
}

type TCPListener struct {
	listener net.Listener
	connChan chan net.Conn
}

func (l *TCPListener) Close() error {
	return l.listener.Close()
}

func (l *TCPListener) AcceptConn() (conn net.Conn, err error) {
	conn = <-l.connChan
	return
}

func (l *TCPListener) Serve(ctx context.Context) error {
	keepAccepting(ctx, l.listener, l.connChan)
	return nil
}

func NewTCPListener(ctx context.Context) (Listener, error) {
	options := ctx.Value(C.OPTIONS).(*args.Options)
	lAddr := options.GetLocalAddr()
	ln, err := net.Listen("tcp", lAddr)
	if err != nil {
		return nil, err
	}

	l := &TCPListener{
		listener: &tcpKeepAliveListener{ln.(*net.TCPListener)},
		connChan: make(chan net.Conn, 1024),
	}

	return l, nil
}

type TLSListener struct {
	*TCPListener
}

func NewTLSListener(ctx context.Context) (Listener, error) {
	inner, err := NewTCPListener(ctx)
	if err != nil {
		return nil, err
	}

	tlsConfig, err := buildServerTLSConfig(ctx)
	if err != nil {
		return nil, err
	}

	l := &TLSListener{inner.(*TCPListener)}
	ln := l.listener
	l.listener = tls.NewListener(ln, tlsConfig)
	return l, nil
}

type MTLSListener struct {
	*TLSListener
}

func (l *MTLSListener) Serve(ctx context.Context) error {
	keepMuxAccepting(ctx, l.listener, l.connChan)
	return nil
}

func NewMTLSListener(ctx context.Context) (Listener, error) {
	inner, err := NewTLSListener(ctx)
	if err != nil {
		return nil, err
	}

	return &MTLSListener{inner.(*TLSListener)}, nil
}

type WSListener struct {
	listener net.Listener
	upgrader *websocket.Upgrader
	server   *http.Server
	connChan chan net.Conn
}

func (l *WSListener) Close() error {
	return l.listener.Close()
}

func (l *WSListener) Serve(ctx context.Context) error {
	return l.server.Serve(l.listener)
}

func (l *WSListener) AcceptConn() (net.Conn, error) {
	return <-l.connChan, nil
}

func (l *WSListener) Upgrade(w http.ResponseWriter, r *http.Request) {
	conn, err := l.upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Errorln("failed to upgrade to websocket: %s", err)
		return
	}

	select {
	case l.connChan <- WebsocketServerConn(conn):
	default:
		log.Warnln("connection queue is full")
		conn.Close()
	}
}

func NewWSListener(ctx context.Context) (Listener, error) {
	options := ctx.Value(C.OPTIONS).(*args.Options)

	lAddr := options.GetLocalAddr()
	ln, err := net.Listen("tcp", lAddr)
	if err != nil {
		return nil, err
	}

	l := &WSListener{
		listener: &tcpKeepAliveListener{ln.(*net.TCPListener)},
		connChan: make(chan net.Conn, 1024),
	}

	l.upgrader = &websocket.Upgrader{
		ReadBufferSize:   4 * 1024,
		WriteBufferSize:  4 * 1024,
		HandshakeTimeout: time.Second * 4,
		CheckOrigin: func(r *http.Request) bool {
			return true
		},
		EnableCompression: !options.Nocomp,
	}

	mux := http.NewServeMux()
	mux.Handle(options.Path, http.HandlerFunc(l.Upgrade))
	l.server = &http.Server{
		Addr:              lAddr,
		Handler:           mux,
		ReadHeaderTimeout: 30 * time.Second,
		MaxHeaderBytes:    2048,
	}

	return l, nil
}

type WSSListener struct {
	*WSListener
}

func NewWSSListener(ctx context.Context) (Listener, error) {
	inner, err := NewWSListener(ctx)
	if err != nil {
		return nil, err
	}

	tlsConfig, err := buildServerTLSConfig(ctx)
	if err != nil {
		return nil, err
	}

	l := &WSSListener{inner.(*WSListener)}
	ln := l.listener
	l.listener = tls.NewListener(ln, tlsConfig) // turn listener into tls.Listener
	return l, nil
}

type MWSListener struct {
	*WSListener
}

func (l *MWSListener) Upgrade(w http.ResponseWriter, r *http.Request) {
	conn, err := l.upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Errorln("failed to upgrade to websocket: %s", err)
		return
	}

	go serveMux(WebsocketServerConn(conn), l.connChan)
}

func NewMWSListener(ctx context.Context) (Listener, error) {
	inner, err := NewWSListener(ctx)
	if err != nil {
		return nil, err
	}

	l := &MWSListener{inner.(*WSListener)}
	options := ctx.Value(C.OPTIONS).(*args.Options)
	mux := http.NewServeMux()
	mux.Handle(options.Path, http.HandlerFunc(l.Upgrade))
	l.server.Handler = mux // replace handler
	return l, nil
}

type MWSSListener struct {
	*MWSListener
}

func NewMWSSListener(ctx context.Context) (Listener, error) {
	inner, err := NewMWSListener(ctx)
	if err != nil {
		return nil, err
	}

	tlsConfig, err := buildServerTLSConfig(ctx)
	if err != nil {
		return nil, err
	}

	l := &MWSSListener{inner.(*MWSListener)}
	ln := l.listener
	l.listener = tls.NewListener(ln, tlsConfig) // turn listener into tls.Listener
	return l, nil
}

func buildServerTLSConfig(ctx context.Context) (*tls.Config, error) {
	options := ctx.Value(C.OPTIONS).(*args.Options)
	cert, err := tls.LoadX509KeyPair(options.Cert, options.Key)
	if err != nil {
		return nil, err
	}

	return &tls.Config{Certificates: []tls.Certificate{cert}}, nil
}

func keepAccepting(ctx context.Context, listener net.Listener, connChan chan net.Conn) {
	for {
		conn, err := listener.Accept()
		if err != nil {
			if errors.IsClosed(err) {
				break
			}

			log.Errorln("failed to accept connection: %s", err)

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

			log.Errorln("failed to accept connection: %s", err)

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
			log.Errorln("failed to accept smux session: %s", err)
		}

		return
	}

	defer session.Close()

	var stream *smux.Stream
	for {
		stream, err = session.AcceptStream()
		if err != nil {
			if !errors.IsEOF(err) && !errors.IsClosed(err) {
				log.Errorln("failed to accept smux stream: %s", err)
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
