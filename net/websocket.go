package net

import (
	"context"
	"net"
	"net/http"
	"time"

	"github.com/gorilla/websocket"
	"github.com/maskedeken/gost-plugin/args"
	C "github.com/maskedeken/gost-plugin/constant"
)

type WebsocketConn struct {
	conn *websocket.Conn
	rb   []byte
}

func WebsocketClientConn(ctx context.Context, conn net.Conn) (net.Conn, error) {
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
	c, resp, err := dialer.Dial(url, header)
	if err != nil {
		return nil, err
	}
	resp.Body.Close()
	return &WebsocketConn{conn: c}, nil
}

func WebsocketServerConn(conn *websocket.Conn) net.Conn {
	// conn.EnableWriteCompression(true)
	return &WebsocketConn{
		conn: conn,
	}
}

func (c *WebsocketConn) Read(b []byte) (n int, err error) {
	if len(c.rb) == 0 {
		_, c.rb, err = c.conn.ReadMessage()
	}
	n = copy(b, c.rb)
	c.rb = c.rb[n:]
	return
}

func (c *WebsocketConn) Write(b []byte) (n int, err error) {
	err = c.conn.WriteMessage(websocket.BinaryMessage, b)
	n = len(b)
	return
}

func (c *WebsocketConn) Close() error {
	return c.conn.Close()
}

func (c *WebsocketConn) LocalAddr() net.Addr {
	return c.conn.LocalAddr()
}

func (c *WebsocketConn) RemoteAddr() net.Addr {
	return c.conn.RemoteAddr()
}

func (c *WebsocketConn) SetDeadline(t time.Time) error {
	if err := c.SetReadDeadline(t); err != nil {
		return err
	}
	return c.SetWriteDeadline(t)
}
func (c *WebsocketConn) SetReadDeadline(t time.Time) error {
	return c.conn.SetReadDeadline(t)
}

func (c *WebsocketConn) SetWriteDeadline(t time.Time) error {
	return c.conn.SetWriteDeadline(t)
}
