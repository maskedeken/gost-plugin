package main

import (
	"context"
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

	muxConn := &muxConn{session.conn, session}
	return muxConn, nil
}

func (tr *mwsTransporter) Handshake(conn net.Conn, options ...gost.HandshakeOption) (net.Conn, error) {
	opts := &gost.HandshakeOptions{}
	for _, option := range options {
		option(opts)
	}

	timeout := opts.Timeout
	if timeout <= 0 {
		timeout = gost.HandshakeTimeout
	}

	conn.SetDeadline(time.Now().Add(timeout))
	defer conn.SetDeadline(time.Time{})

	mConn := conn.(*muxConn)
	cc, err := mConn.session.GetConn()
	if err != nil {
		tr.Lock()
		defer tr.Unlock()

		mConn.session.Close()
		tr.sessionManager.Remove(mConn.session)
		return nil, err
	}
	return cc, nil
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

	muxConn := &muxConn{session.conn, session}
	return muxConn, nil
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

type wsListener struct {
	addr     net.Addr
	upgrader *websocket.Upgrader
	srv      *http.Server
	connChan chan net.Conn
	errChan  chan error
}

// WSListener creates a Listener for websocket proxy server.
func WSListener(addr string, options *WSOptions) (gost.Listener, error) {
	if options == nil {
		options = &WSOptions{}
	}
	l := &wsListener{
		upgrader: &websocket.Upgrader{
			ReadBufferSize:    options.ReadBufferSize,
			WriteBufferSize:   options.WriteBufferSize,
			CheckOrigin:       func(r *http.Request) bool { return true },
			EnableCompression: options.EnableCompression,
		},
		connChan: make(chan net.Conn, 1024),
		errChan:  make(chan error, 1),
	}

	path := options.Path
	if path == "" {
		path = defaultWSPath
	}
	mux := http.NewServeMux()
	mux.Handle(path, http.HandlerFunc(l.upgrade))
	l.srv = &http.Server{
		Addr:              addr,
		Handler:           mux,
		ReadHeaderTimeout: 30 * time.Second,
	}

	listenCfg := &net.ListenConfig{Control: getListenerControlFunc(options)}
	ln, err := listenCfg.Listen(context.Background(), "tcp", addr)
	if err != nil {
		return nil, err
	}
	l.addr = ln.Addr()
	tcpListener, _ := ln.(*net.TCPListener)

	go func() {
		err := l.srv.Serve(tcpKeepAliveListener{tcpListener})
		if err != nil {
			l.errChan <- err
		}
		close(l.errChan)
	}()
	select {
	case err := <-l.errChan:
		return nil, err
	default:
	}

	return l, nil
}

func (l *wsListener) upgrade(w http.ResponseWriter, r *http.Request) {
	conn, err := l.upgrader.Upgrade(w, r, nil)
	if err != nil {
		Logf("[ws] %s - %s : %s", r.RemoteAddr, l.addr, err)
		return
	}
	select {
	case l.connChan <- websocketServerConn(conn):
	default:
		conn.Close()
		Logf("[ws] %s - %s: connection queue is full", r.RemoteAddr, l.addr)
	}
}

func (l *wsListener) Accept() (conn net.Conn, err error) {
	select {
	case conn = <-l.connChan:
	case err = <-l.errChan:
	}
	return
}

func (l *wsListener) Close() error {
	return l.srv.Close()
}

func (l *wsListener) Addr() net.Addr {
	return l.addr
}

type mwsListener struct {
	addr     net.Addr
	upgrader *websocket.Upgrader
	srv      *http.Server
	connChan chan net.Conn
	errChan  chan error
}

// MWSListener creates a Listener for multiplex-websocket proxy server.
func MWSListener(addr string, options *WSOptions) (gost.Listener, error) {
	if options == nil {
		options = &WSOptions{}
	}
	l := &mwsListener{
		upgrader: &websocket.Upgrader{
			ReadBufferSize:    options.ReadBufferSize,
			WriteBufferSize:   options.WriteBufferSize,
			CheckOrigin:       func(r *http.Request) bool { return true },
			EnableCompression: options.EnableCompression,
		},
		connChan: make(chan net.Conn, 1024),
		errChan:  make(chan error, 1),
	}

	path := options.Path
	if path == "" {
		path = defaultWSPath
	}

	mux := http.NewServeMux()
	mux.Handle(path, http.HandlerFunc(l.upgrade))
	l.srv = &http.Server{
		Addr:              addr,
		Handler:           mux,
		ReadHeaderTimeout: 30 * time.Second,
	}

	listenCfg := &net.ListenConfig{Control: getListenerControlFunc(options)}
	ln, err := listenCfg.Listen(context.Background(), "tcp", addr)
	if err != nil {
		return nil, err
	}
	l.addr = ln.Addr()
	tcpListener, _ := ln.(*net.TCPListener)

	go func() {
		err := l.srv.Serve(tcpKeepAliveListener{tcpListener})
		if err != nil {
			l.errChan <- err
		}
		close(l.errChan)
	}()
	select {
	case err := <-l.errChan:
		return nil, err
	default:
	}

	return l, nil
}

func (l *mwsListener) upgrade(w http.ResponseWriter, r *http.Request) {
	conn, err := l.upgrader.Upgrade(w, r, nil)
	if err != nil {
		Logf("[mws] %s - %s : %s", r.RemoteAddr, l.addr, err)
		return
	}

	l.mux(websocketServerConn(conn))
}

func (l *mwsListener) mux(conn net.Conn) {
	smuxConfig := smux.DefaultConfig()
	mux, err := smux.Server(conn, smuxConfig)
	if err != nil {
		Logf("[mws] %s - %s : %s", conn.RemoteAddr(), l.Addr(), err)
		return
	}
	defer mux.Close()

	for {
		stream, err := mux.AcceptStream()
		if err != nil {
			Logf("[mws] accept stream:", err)
			return
		}

		cc := &muxStreamConn{Conn: conn, stream: stream}
		select {
		case l.connChan <- cc:
		default:
			cc.Close()
			Logf("[mws] %s - %s: connection queue is full", conn.RemoteAddr(), conn.LocalAddr())
		}
	}
}

func (l *mwsListener) Accept() (conn net.Conn, err error) {
	select {
	case conn = <-l.connChan:
	case err = <-l.errChan:
	}
	return
}

func (l *mwsListener) Close() error {
	return l.srv.Close()
}

func (l *mwsListener) Addr() net.Addr {
	return l.addr
}

type wssListener struct {
	*wsListener
}

// WSSListener creates a Listener for websocket secure proxy server.
func WSSListener(addr string, tlsConfig *tls.Config, options *WSOptions) (gost.Listener, error) {
	if options == nil {
		options = &WSOptions{}
	}
	l := &wssListener{
		wsListener: &wsListener{
			upgrader: &websocket.Upgrader{
				ReadBufferSize:    options.ReadBufferSize,
				WriteBufferSize:   options.WriteBufferSize,
				CheckOrigin:       func(r *http.Request) bool { return true },
				EnableCompression: options.EnableCompression,
			},
			connChan: make(chan net.Conn, 1024),
			errChan:  make(chan error, 1),
		},
	}

	if tlsConfig == nil {
		tlsConfig = gost.DefaultTLSConfig
	}

	path := options.Path
	if path == "" {
		path = defaultWSPath
	}

	mux := http.NewServeMux()
	mux.Handle(path, http.HandlerFunc(l.upgrade))
	l.srv = &http.Server{
		Addr:              addr,
		TLSConfig:         tlsConfig,
		Handler:           mux,
		ReadHeaderTimeout: 30 * time.Second,
	}

	listenCfg := &net.ListenConfig{Control: getListenerControlFunc(options)}
	ln, err := listenCfg.Listen(context.Background(), "tcp", addr)
	if err != nil {
		return nil, err
	}
	l.addr = ln.Addr()
	tcpListener, _ := ln.(*net.TCPListener)

	go func() {
		err := l.srv.Serve(tls.NewListener(tcpKeepAliveListener{tcpListener}, tlsConfig))
		if err != nil {
			l.errChan <- err
		}
		close(l.errChan)
	}()
	select {
	case err := <-l.errChan:
		return nil, err
	default:
	}

	return l, nil
}

type mwssListener struct {
	*mwsListener
}

// MWSSListener creates a Listener for multiplex-websocket secure proxy server.
func MWSSListener(addr string, tlsConfig *tls.Config, options *WSOptions) (gost.Listener, error) {
	if options == nil {
		options = &WSOptions{}
	}
	l := &mwssListener{
		mwsListener: &mwsListener{
			upgrader: &websocket.Upgrader{
				ReadBufferSize:    options.ReadBufferSize,
				WriteBufferSize:   options.WriteBufferSize,
				CheckOrigin:       func(r *http.Request) bool { return true },
				EnableCompression: options.EnableCompression,
			},
			connChan: make(chan net.Conn, 1024),
			errChan:  make(chan error, 1),
		},
	}

	if tlsConfig == nil {
		tlsConfig = gost.DefaultTLSConfig
	}

	path := options.Path
	if path == "" {
		path = defaultWSPath
	}

	mux := http.NewServeMux()
	mux.Handle(path, http.HandlerFunc(l.upgrade))
	l.srv = &http.Server{
		Addr:              addr,
		TLSConfig:         tlsConfig,
		Handler:           mux,
		ReadHeaderTimeout: 30 * time.Second,
	}

	listenCfg := &net.ListenConfig{Control: getListenerControlFunc(options)}
	ln, err := listenCfg.Listen(context.Background(), "tcp", addr)
	if err != nil {
		return nil, err
	}
	l.addr = ln.Addr()
	tcpListener, _ := ln.(*net.TCPListener)

	go func() {
		err := l.srv.Serve(tls.NewListener(tcpKeepAliveListener{tcpListener}, tlsConfig))
		if err != nil {
			l.errChan <- err
		}
		close(l.errChan)
	}()
	select {
	case err := <-l.errChan:
		return nil, err
	default:
	}

	return l, nil
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
