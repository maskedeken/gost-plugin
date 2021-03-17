package protocol

import (
	"context"
	"crypto/tls"
	"encoding/base64"
	"io"
	"net"
	"net/http"
	"time"

	"github.com/gorilla/websocket"
	"github.com/maskedeken/gost-plugin/args"
	C "github.com/maskedeken/gost-plugin/constant"
	"github.com/maskedeken/gost-plugin/gost"
	"github.com/maskedeken/gost-plugin/log"
	"github.com/maskedeken/gost-plugin/registry"
)

// WSListener is Listener which handles websocket protocol
type WSListener struct {
	*TCPListener
	upgrader *websocket.Upgrader
	server   *http.Server
	path     string
}

// Close implements gost.Listener.Close()
func (l *WSListener) Close() error {
	return l.listener.Close()
}

// Serve implements gost.Listener.Serve()
func (l *WSListener) Serve(ctx context.Context) error {
	return l.server.Serve(l.listener)
}

// AcceptConn implements gost.Listener.AcceptConn()
func (l *WSListener) AcceptConn() (net.Conn, error) {
	return <-l.connChan, nil
}

// Upgrade turns net.Conn into websocket conn
func (l *WSListener) Upgrade(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != l.path {
		w.WriteHeader(http.StatusNotFound)
		return
	}

	var err error
	conn, err := l.upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Errorf("failed to upgrade to websocket: %s", err)
		return
	}

	var ed []byte
	if str := r.Header.Get("Sec-WebSocket-Protocol"); str != "" {
		if ed, err = base64.StdEncoding.DecodeString(str); err != nil {
			log.Errorf("failed to decode the early payload: %s", err)
		}
	}

	select {
	case l.connChan <- websocketServerConn(conn, ed):
	default:
		log.Warnln("connection queue is full")
		conn.Close()
	}
}

// NewWSListener is the constructor for WSListener
func NewWSListener(ctx context.Context) (gost.Listener, error) {
	inner, err := NewTCPListener(ctx)
	if err != nil {
		return nil, err
	}

	options := ctx.Value(C.OPTIONS).(*args.Options)
	l := &WSListener{
		TCPListener: inner.(*TCPListener),
		path:        options.Path,
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

	lAddr := options.GetLocalAddr()
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

// WSSListener is Listener which handles websocket over tls
type WSSListener struct {
	*WSListener
}

// NewWSSListener is the constructor for WSSListener
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

// WSTransporter is Transporter which handles websocket protocol
type WSTransporter struct {
	*TCPTransporter
}

// DialConn implements gost.Transporter.DialConn()
func (t *WSTransporter) DialConn() (net.Conn, error) {
	wsConn, err := dialWS(t.ctx, t.TCPTransporter.DialConn)
	if err != nil {
		return nil, err
	}

	return wsConn, nil
}

// NewWSTransporter is constructor for WSTransporter
func NewWSTransporter(ctx context.Context) (gost.Transporter, error) {
	inner, err := NewTCPTransporter(ctx)
	if err != nil {
		return nil, err
	}

	return &WSTransporter{inner.(*TCPTransporter)}, nil
}

// WSSTransporter is Transporter which handles websocket over tls
type WSSTransporter struct {
	*TLSTransporter
}

// DialConn implements gost.Transporter.DialConn()
func (t *WSSTransporter) DialConn() (net.Conn, error) {
	wsConn, err := dialWS(t.ctx, t.TLSTransporter.DialConn)
	if err != nil {
		return nil, err
	}

	return wsConn, nil
}

// NewWSSTransporter is constructor for WSSTransporter
func NewWSSTransporter(ctx context.Context) (gost.Transporter, error) {
	inner, err := NewTLSTransporter(ctx)
	if err != nil {
		return nil, err
	}

	return &WSSTransporter{inner.(*TLSTransporter)}, nil
}

func dialWS(ctx context.Context, newConn func() (net.Conn, error)) (net.Conn, error) {
	options := ctx.Value(C.OPTIONS).(*args.Options)
	rAddr := options.GetRemoteAddr()
	ed := options.Ed
	url := "ws://" + rAddr + options.Path

	if ed > 0 {
		return &delayDialConn{
			ctx:     ctx,
			url:     url,
			newConn: newConn,
			ed:      ed,
			dialed:  make(chan struct{}),
		}, nil
	}

	return websocketClientConn(ctx, url, newConn, nil)
}

type delayDialConn struct {
	net.Conn
	ctx     context.Context
	url     string
	newConn func() (net.Conn, error)
	ed      uint
	dialed  chan struct{}
	closed  bool
}

func (d *delayDialConn) Write(b []byte) (int, error) {
	if d.closed {
		return 0, io.ErrClosedPipe
	}

	if d.Conn == nil {
		ed := b
		if len(ed) > int(d.ed) {
			ed = nil
		}
		var err error
		if d.Conn, err = websocketClientConn(d.ctx, d.url, d.newConn, ed); err != nil {
			d.Close()
			return 0, err
		}
		close(d.dialed)
		if ed != nil {
			return len(ed), nil
		}
	}

	return d.Conn.Write(b)
}

func (d *delayDialConn) Read(b []byte) (int, error) {
	<-d.dialed

	if d.closed {
		return 0, io.ErrClosedPipe
	}

	return d.Conn.Read(b)
}

func (d *delayDialConn) Close() error {
	d.closed = true
	if d.Conn == nil {
		return nil
	}
	return d.Conn.Close()
}

type websocketConn struct {
	conn *websocket.Conn
	rb   []byte
}

func websocketClientConn(ctx context.Context, url string, newConn func() (net.Conn, error), ed []byte) (net.Conn, error) {
	options := ctx.Value(C.OPTIONS).(*args.Options)
	dialer := websocket.Dialer{
		ReadBufferSize:    4 * 1024,
		WriteBufferSize:   4 * 1024,
		HandshakeTimeout:  time.Second * 30,
		EnableCompression: !options.Nocomp,
		NetDial: func(net, addr string) (net.Conn, error) {
			return newConn()
		},
	}

	header := http.Header{}
	header.Set("User-Agent", C.DEFAULT_USER_AGENT)
	if options.Hostname != "" {
		header.Set("Host", options.Hostname)
	}

	if ed != nil {
		header.Set("Sec-WebSocket-Protocol", base64.StdEncoding.EncodeToString(ed))
	}

	c, resp, err := dialer.Dial(url, header)
	if err != nil {
		return nil, err
	}
	resp.Body.Close()
	return &websocketConn{conn: c}, nil
}

func websocketServerConn(conn *websocket.Conn, ed []byte) net.Conn {
	// conn.EnableWriteCompression(true)
	return &websocketConn{
		conn: conn,
		rb:   ed,
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
}
