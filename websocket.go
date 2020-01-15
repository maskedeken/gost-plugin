package main

import (
	"crypto/tls"
	"net"
	"net/http"
	"net/url"
	"sync"
	"time"

	"github.com/ginuerzh/gost"
	"gopkg.in/gorilla/websocket.v1"
	"gopkg.in/xtaci/smux.v1"
)

const (
	defaultWSPath = "/ws"
)

type WSOptions struct {
	gost.WSOptions
	node        *gost.Node
	Fastopen    bool
	muxSessions uint16
}

type wsTransporter struct {
	options *WSOptions
}

func WSTransporter(wsopts *WSOptions) gost.Transporter {
	return &wsTransporter{options: wsopts}
}

func (tr *wsTransporter) Dial(addr string, options ...gost.DialOption) (net.Conn, error) {
	opts := &gost.DialOptions{}
	for _, option := range options {
		option(opts)
	}

	timeout := opts.Timeout
	if timeout <= 0 {
		timeout = gost.DialTimeout
	}
	if opts.Chain == nil {
		dialer := &net.Dialer{Timeout: timeout,
			Control: getDialerControlFunc(tr.options)}
		return dialer.Dial("tcp", addr)
	}
	return opts.Chain.Dial(addr)
}

func (tr *wsTransporter) Multiplex() bool {
	return false
}

func (tr *wsTransporter) Handshake(conn net.Conn, options ...gost.HandshakeOption) (net.Conn, error) {
	opts := &gost.HandshakeOptions{}
	for _, option := range options {
		option(opts)
	}
	wsOptions := tr.options
	if wsOptions == nil {
		wsOptions = &WSOptions{}
	}

	path := wsOptions.Path
	if path == "" {
		path = defaultWSPath
	}
	url := url.URL{Scheme: "ws", Host: opts.Host, Path: path}
	return websocketClientConn(url.String(), conn, nil, wsOptions)
}

type mwsTransporter struct {
	sync.Mutex
	options        *WSOptions
	sessionManager *sessionManager
}

func MWSTransporter(opts *WSOptions) gost.Transporter {
	return &mwsTransporter{
		options:        opts,
		sessionManager: SessionManager(opts.muxSessions),
	}
}

func (tr *mwsTransporter) Dial(addr string, options ...gost.DialOption) (conn net.Conn, err error) {
	opts := &gost.DialOptions{}
	for _, option := range options {
		option(opts)
	}

	node := tr.options.node
	handshakeOpts := &gost.HandshakeOptions{}
	for _, option := range node.HandshakeOptions {
		option(handshakeOpts)
	}

	tr.Lock()
	defer tr.Unlock()

	session := tr.sessionManager.Get()
	if session != nil && session.IsClosed() {
		tr.sessionManager.Remove(session)
		session = nil
	}
	if session == nil {
		timeout := opts.Timeout
		if timeout <= 0 {
			timeout = gost.DialTimeout
		}

		if opts.Chain == nil {
			dialer := &net.Dialer{Timeout: timeout,
				Control: getDialerControlFunc(tr.options)}
			conn, err = dialer.Dial("tcp", addr)
		} else {
			conn, err = opts.Chain.Dial(addr)
		}
		if err != nil {
			return
		}

		session, err = tr.initSession(node.Addr, conn, handshakeOpts)
		if err != nil {
			conn.Close()
			return nil, err
		}
	}

	cc, err := session.GetConn()
	if err != nil {
		session.Close()
		tr.sessionManager.Remove(session)
		return nil, err
	}
	return cc, nil
}

func (tr *mwsTransporter) Handshake(conn net.Conn, options ...gost.HandshakeOption) (net.Conn, error) {
	return conn, nil
}

func (tr *mwsTransporter) initSession(addr string, conn net.Conn, opts *gost.HandshakeOptions) (*muxSession, error) {
	if opts == nil {
		opts = &gost.HandshakeOptions{}
	}

	wsOptions := tr.options
	path := wsOptions.Path
	if path == "" {
		path = defaultWSPath
	}
	url := url.URL{Scheme: "ws", Host: opts.Host, Path: path}
	conn, err := websocketClientConn(url.String(), conn, nil, wsOptions)
	if err != nil {
		return nil, err
	}
	// stream multiplex
	smuxConfig := smux.DefaultConfig()
	session, err := smux.Client(conn, smuxConfig)
	if err != nil {
		return nil, err
	}
	return tr.sessionManager.Create(conn, session), nil
}

func (tr *mwsTransporter) Multiplex() bool {
	return true
}

type wssTransporter struct {
	wsTransporter
}

// WSSTransporter creates a Transporter that is used by websocket secure proxy client.
func WSSTransporter(opts *WSOptions) gost.Transporter {
	return &wssTransporter{
		wsTransporter{options: opts},
	}
}

func (tr *wssTransporter) Handshake(conn net.Conn, options ...gost.HandshakeOption) (net.Conn, error) {
	opts := &gost.HandshakeOptions{}
	for _, option := range options {
		option(opts)
	}
	wsOptions := tr.options

	if opts.TLSConfig == nil {
		opts.TLSConfig = &tls.Config{InsecureSkipVerify: true}
	}
	path := wsOptions.Path
	if path == "" {
		path = defaultWSPath
	}
	url := url.URL{Scheme: "wss", Host: opts.Host, Path: path}
	return websocketClientConn(url.String(), conn, opts.TLSConfig, wsOptions)
}

type mwssTransporter struct {
	mwsTransporter
}

// MWSSTransporter creates a Transporter that is used by multiplex-websocket secure proxy client.
func MWSSTransporter(opts *WSOptions) gost.Transporter {
	return &mwssTransporter{
		mwsTransporter{options: opts,
			sessionManager: SessionManager(opts.muxSessions)},
	}
}

func (tr *mwssTransporter) Dial(addr string, options ...gost.DialOption) (conn net.Conn, err error) {
	opts := &gost.DialOptions{}
	for _, option := range options {
		option(opts)
	}

	node := tr.options.node
	handshakeOpts := &gost.HandshakeOptions{}
	for _, option := range node.HandshakeOptions {
		option(handshakeOpts)
	}

	tr.Lock()
	defer tr.Unlock()

	session := tr.sessionManager.Get()
	if session != nil && session.IsClosed() {
		tr.sessionManager.Remove(session)
		session = nil
	}
	if session == nil {
		timeout := opts.Timeout
		if timeout <= 0 {
			timeout = gost.DialTimeout
		}

		if opts.Chain == nil {
			dialer := &net.Dialer{Timeout: timeout,
				Control: getDialerControlFunc(tr.options)}
			conn, err = dialer.Dial("tcp", addr)
		} else {
			conn, err = opts.Chain.Dial(addr)
		}
		if err != nil {
			return
		}

		session, err = tr.initSession(node.Addr, conn, handshakeOpts)
		if err != nil {
			conn.Close()
			return nil, err
		}
	}

	cc, err := session.GetConn()
	if err != nil {
		session.Close()
		tr.sessionManager.Remove(session)
		return nil, err
	}
	return cc, nil
}

func (tr *mwssTransporter) initSession(addr string, conn net.Conn, opts *gost.HandshakeOptions) (*muxSession, error) {
	if opts == nil {
		opts = &gost.HandshakeOptions{}
	}
	wsOptions := tr.options

	tlsConfig := opts.TLSConfig
	if tlsConfig == nil {
		tlsConfig = &tls.Config{InsecureSkipVerify: true}
	}
	path := wsOptions.Path
	if path == "" {
		path = defaultWSPath
	}
	url := url.URL{Scheme: "wss", Host: opts.Host, Path: path}
	conn, err := websocketClientConn(url.String(), conn, tlsConfig, wsOptions)
	if err != nil {
		return nil, err
	}
	// stream multiplex
	smuxConfig := smux.DefaultConfig()
	session, err := smux.Client(conn, smuxConfig)
	if err != nil {
		return nil, err
	}
	return tr.sessionManager.Create(conn, session), nil
}

type websocketConn struct {
	conn *websocket.Conn
	rb   []byte
}

func websocketClientConn(url string, conn net.Conn, tlsConfig *tls.Config, options *WSOptions) (net.Conn, error) {
	if options == nil {
		options = &WSOptions{}
	}

	timeout := options.HandshakeTimeout
	if timeout <= 0 {
		timeout = gost.HandshakeTimeout
	}

	dialer := websocket.Dialer{
		ReadBufferSize:    options.ReadBufferSize,
		WriteBufferSize:   options.WriteBufferSize,
		TLSClientConfig:   tlsConfig,
		HandshakeTimeout:  timeout,
		EnableCompression: options.EnableCompression,
		NetDial: func(net, addr string) (net.Conn, error) {
			return conn, nil
		},
	}
	header := http.Header{}
	header.Set("User-Agent", gost.DefaultUserAgent)
	if options.UserAgent != "" {
		header.Set("User-Agent", options.UserAgent)
	}
	c, resp, err := dialer.Dial(url, header)
	if err != nil {
		return nil, err
	}
	resp.Body.Close()
	return &websocketConn{conn: c}, nil
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
