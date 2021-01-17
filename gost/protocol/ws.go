package protocol

import (
	"context"
	"crypto/tls"
	"net"
	"net/http"
	"time"

	"github.com/gorilla/websocket"
	"github.com/maskedeken/gost-plugin/args"
	C "github.com/maskedeken/gost-plugin/constant"
	"github.com/maskedeken/gost-plugin/gost"
	"github.com/maskedeken/gost-plugin/log"
	"github.com/maskedeken/gost-plugin/mux"
	"github.com/maskedeken/gost-plugin/registry"
)

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
		log.Errorf("failed to upgrade to websocket: %s", err)
		return
	}

	select {
	case l.connChan <- websocketServerConn(conn):
	default:
		log.Warnln("connection queue is full")
		conn.Close()
	}
}

func NewWSListener(ctx context.Context) (gost.Listener, error) {
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

func NewWSSListener(ctx context.Context) (gost.Listener, error) {
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
		log.Errorf("failed to upgrade to websocket: %s", err)
		return
	}

	go serveMux(websocketServerConn(conn), l.connChan)
}

func NewMWSListener(ctx context.Context) (gost.Listener, error) {
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

func NewMWSSListener(ctx context.Context) (gost.Listener, error) {
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

type WSTransporter struct {
	*TCPTransporter
}

func (t *WSTransporter) DialConn() (net.Conn, error) {
	conn, err := t.TCPTransporter.DialConn()
	if err != nil {
		return nil, err
	}

	wsConn, err := websocketClientConn(t.ctx, conn)
	if err != nil {
		return nil, err
	}

	return wsConn, nil
}

func NewWSTransporter(ctx context.Context) (gost.Transporter, error) {
	inner, err := NewTCPTransporter(ctx)
	if err != nil {
		return nil, err
	}

	return &WSTransporter{inner.(*TCPTransporter)}, nil
}

type WSSTransporter struct {
	*TLSTransporter
}

func (t *WSSTransporter) DialConn() (net.Conn, error) {
	conn, err := t.TLSTransporter.DialConn()
	if err != nil {
		return nil, err
	}

	wsConn, err := websocketClientConn(t.ctx, conn)
	if err != nil {
		return nil, err
	}

	return wsConn, nil
}

func NewWSSTransporter(ctx context.Context) (gost.Transporter, error) {
	inner, err := NewTLSTransporter(ctx)
	if err != nil {
		return nil, err
	}

	return &WSSTransporter{inner.(*TLSTransporter)}, nil
}

type MWSTransporter struct {
	*WSTransporter
	pool *mux.MuxPool
}

func (t *MWSTransporter) DialConn() (net.Conn, error) {
	return t.pool.DialMux(t.WSTransporter.DialConn)
}

func NewMWSTransporter(ctx context.Context) (gost.Transporter, error) {
	inner, err := NewWSTransporter(ctx)
	if err != nil {
		return nil, err
	}

	return &MWSTransporter{WSTransporter: inner.(*WSTransporter),
		pool: mux.NewMuxPool(ctx),
	}, nil
}

type MWSSTransporter struct {
	*WSSTransporter
	pool *mux.MuxPool
}

func (t *MWSSTransporter) DialConn() (net.Conn, error) {
	return t.pool.DialMux(t.WSSTransporter.DialConn)
}

func NewMWSSTransporter(ctx context.Context) (gost.Transporter, error) {
	inner, err := NewWSSTransporter(ctx)
	if err != nil {
		return nil, err
	}

	return &MWSSTransporter{WSSTransporter: inner.(*WSSTransporter),
		pool: mux.NewMuxPool(ctx),
	}, nil
}

type websocketConn struct {
	conn *websocket.Conn
	rb   []byte
}

func websocketClientConn(ctx context.Context, conn net.Conn) (net.Conn, error) {
	options := ctx.Value(C.OPTIONS).(*args.Options)
	rAddr := options.GetRemoteAddr()
	url := "ws://" + rAddr + options.Path

	dialer := websocket.Dialer{
		ReadBufferSize:    4 * 1024,
		WriteBufferSize:   4 * 1024,
		HandshakeTimeout:  time.Second * 30,
		EnableCompression: !options.Nocomp,
		NetDial: func(net, addr string) (net.Conn, error) {
			return conn, nil
		},
	}

	header := http.Header{}
	header.Set("User-Agent", C.DEFAULT_USER_AGENT)
	if options.Hostname != "" {
		header.Set("Host", options.Hostname)
	}
	c, resp, err := dialer.Dial(url, header)
	if err != nil {
		return nil, err
	}
	resp.Body.Close()
	return &websocketConn{conn: c}, nil
}

func websocketServerConn(conn *websocket.Conn) net.Conn {
	// conn.EnableWriteCompression(true)
	return &websocketConn{
		conn: conn,
	}
}

func (c *websocketConn) Read(b []byte) (n int, err error) {
	if len(c.rb) == 0 {
		_, c.rb, err = c.conn.ReadMessage()
	}
	n = copy(b, c.rb)
	c.rb = c.rb[n:]
	return
}

func (c *websocketConn) Write(b []byte) (n int, err error) {
	err = c.conn.WriteMessage(websocket.BinaryMessage, b)
	n = len(b)
	return
}

func (c *websocketConn) Close() error {
	return c.conn.Close()
}

func (c *websocketConn) LocalAddr() net.Addr {
	return c.conn.LocalAddr()
}

func (c *websocketConn) RemoteAddr() net.Addr {
	return c.conn.RemoteAddr()
}

func (c *websocketConn) SetDeadline(t time.Time) error {
	if err := c.SetReadDeadline(t); err != nil {
		return err
	}
	return c.SetWriteDeadline(t)
}
func (c *websocketConn) SetReadDeadline(t time.Time) error {
	return c.conn.SetReadDeadline(t)
}

func (c *websocketConn) SetWriteDeadline(t time.Time) error {
	return c.conn.SetWriteDeadline(t)
}

func init() {
	registry.RegisterListener("ws", NewWSListener)
	registry.RegisterTransporter("ws", NewWSTransporter)

	registry.RegisterListener("wss", NewWSSListener)
	registry.RegisterTransporter("wss", NewWSSTransporter)

	registry.RegisterListener("mws", NewMWSListener)
	registry.RegisterTransporter("mws", NewMWSTransporter)

	registry.RegisterListener("mwss", NewMWSSListener)
	registry.RegisterTransporter("mwss", NewMWSSTransporter)
}
