package main

import (
	"crypto/tls"
	"net"
	"net/http"
	"net/url"
	"time"

	"github.com/gorilla/websocket"
)

const (
	defaultUserAgent = "Chrome/78.0.3904.106"
)

type requestTransporter interface {
	DialConn() (net.Conn, error)
}

type wsTransporter struct {
	url         url.URL
	tlsConfig   *tls.Config
	compression bool
}

func newWSTransporter(url url.URL, tlsConfig *tls.Config, compression bool) *wsTransporter {
	return &wsTransporter{
		url:         url,
		tlsConfig:   tlsConfig,
		compression: compression,
	}
}

func (t *wsTransporter) DialConn() (net.Conn, error) {
	outbound, err := net.Dial("tcp", t.url.Host)
	if err != nil {
		return nil, err
	}

	return websocketClientConn(t.url.String(), outbound, t.tlsConfig, t.compression)
}

type mwsTransporter struct {
	*wsTransporter
	pool *muxPool
}

func newMWSTransporter(url url.URL, tlsConfig *tls.Config, compression bool, pool *muxPool) *mwsTransporter {
	return &mwsTransporter{
		wsTransporter: newWSTransporter(url, tlsConfig, compression),
		pool:          pool,
	}
}

func (t *mwsTransporter) DialConn() (conn net.Conn, err error) {
	return t.pool.DialMux(t.wsTransporter.DialConn)
}

type websocketConn struct {
	conn *websocket.Conn
	rb   []byte
}

func websocketClientConn(url string, conn net.Conn, tlsConfig *tls.Config, compression bool) (net.Conn, error) {
	dialer := websocket.Dialer{
		ReadBufferSize:    4 * 1024,
		WriteBufferSize:   4 * 1024,
		HandshakeTimeout:  time.Second * 8,
		TLSClientConfig:   tlsConfig,
		EnableCompression: compression,
		NetDial: func(net, addr string) (net.Conn, error) {
			return conn, nil
		},
	}

	header := http.Header{}
	header.Set("User-Agent", defaultUserAgent)
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
