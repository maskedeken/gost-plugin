package client

import (
	"context"
	"encoding/base64"
	"io"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/gorilla/websocket"
	"github.com/maskedeken/gost-plugin/args"
	C "github.com/maskedeken/gost-plugin/constant"
	"github.com/maskedeken/gost-plugin/gost"
	ws "github.com/maskedeken/gost-plugin/gost/protocol/websocket"
	"github.com/maskedeken/gost-plugin/gost/proxy"
	"github.com/maskedeken/gost-plugin/registry"
)

// WSTransporter is Transporter which handles websocket protocol
type WSTransporter struct {
	*proxy.TCPTransporter
}

// DialConn implements gost.Transporter.DialConn()
func (t *WSTransporter) DialConn() (net.Conn, error) {
	options := t.Context.Value(C.OPTIONS).(*args.Options)
	rAddr := options.GetRemoteAddr()
	url := "wss://" + rAddr + options.Path
	wsConn, err := websocketClientConn(t.Context, url, t.TCPTransporter)
	if err != nil {
		return nil, err
	}

	return wsConn, nil
}

// NewWSTransporter is constructor for WSTransporter
func NewWSTransporter(ctx context.Context) (gost.Transporter, error) {
	inner, err := proxy.NewTCPTransporter(ctx)
	if err != nil {
		return nil, err
	}

	return &WSTransporter{inner.(*proxy.TCPTransporter)}, nil
}

// WSSTransporter is Transporter which handles websocket over tls
type WSSTransporter struct {
	*TLSTransporter
}

// DialConn implements gost.Transporter.DialConn()
func (t *WSSTransporter) DialConn() (net.Conn, error) {
	options := t.Context.Value(C.OPTIONS).(*args.Options)
	rAddr := options.GetRemoteAddr()
	url := "wss://" + rAddr + options.Path
	wsConn, err := websocketClientConn(t.Context, url, t.TLSTransporter)
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

func websocketClientConn(ctx context.Context, url string, transporter gost.Transporter) (net.Conn, error) {
	options := ctx.Value(C.OPTIONS).(*args.Options)
	ed := options.Ed

	if ed > 0 {
		return &delayDialConn{
			ctx:         ctx,
			url:         url,
			transporter: transporter,
			ed:          ed,
			dialed:      make(chan struct{}),
		}, nil
	}

	return streamWebsocketConn(ctx, url, transporter, nil)
}

type delayDialConn struct {
	net.Conn
	transporter gost.Transporter
	ctx         context.Context
	url         string
	ed          uint
	dialed      chan struct{}
	closed      bool
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
		if d.Conn, err = streamWebsocketConn(d.ctx, d.url, d.transporter, ed); err != nil {
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

	if d.Conn != nil {
		return d.Conn.Close()
	}

	return nil
}

func streamWebsocketConn(ctx context.Context, url string, transporter gost.Transporter, ed []byte) (net.Conn, error) {
	options := ctx.Value(C.OPTIONS).(*args.Options)
	dialer := websocket.Dialer{
		ReadBufferSize:    4 * 1024,
		WriteBufferSize:   4 * 1024,
		HandshakeTimeout:  time.Second * 8,
		EnableCompression: !options.Nocomp,
		NetDial: func(net, addr string) (net.Conn, error) {
			return transporter.DialConn()
		},
	}

	if strings.HasPrefix(url, "wss") {
		dialer.NetDialTLSContext = func(ctx context.Context, network, addr string) (net.Conn, error) {
			return transporter.DialConn()
		}
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
	return ws.NewWebsocketConn(c, nil), nil
}

func init() {
	registry.RegisterTransporter("ws", NewWSTransporter)
	registry.RegisterTransporter("wss", NewWSSTransporter)
}
