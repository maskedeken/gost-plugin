package main

import (
	"crypto/tls"
	"fmt"
	"net"
	"strings"

	"github.com/ginuerzh/gost"
)

type Router struct {
	server  *gost.Server
	handler gost.Handler
}

type unixTransporter struct {
	unixAddr string
}

func (r *Router) Serve() error {
	return r.server.Serve(r.handler)
}

func (r *Router) Close() error {
	if r == nil || r.server == nil {
		return nil
	}
	return r.server.Close()
}

func UnixTransporter(unixAddr string) gost.Transporter {
	return &unixTransporter{unixAddr: unixAddr}
}

func (ut *unixTransporter) Dial(addr string, options ...gost.DialOption) (net.Conn, error) {
	return net.Dial("unix", ut.unixAddr)
}

func (ut *unixTransporter) Handshake(conn net.Conn, options ...gost.HandshakeOption) (net.Conn, error) {
	return conn, nil
}

func (tr *unixTransporter) Multiplex() bool {
	return false
}

func NewRouter(opts *Options) (*Router, error) {
	if opts.server {
		return configServer(opts)
	} else {
		return configRouter(opts)
	}
}

func configRouter(opts *Options) (*Router, error) {
	chain, err := parseForwardChain(opts)
	if err != nil {
		return nil, err
	}

	url := fmt.Sprintf("tcp://%s:%d", fixAddr(opts.localAddr), opts.localPort)
	node, err := gost.ParseNode(url)
	if err != nil {
		return nil, err
	}

	ln, err := gost.TCPListener(node.Addr)
	if err != nil {
		return nil, err
	}

	handler := gost.TCPDirectForwardHandler(node.Remote)
	handler.Init(
		gost.AddrHandlerOption(ln.Addr().String()),
		gost.ChainHandlerOption(chain),
		gost.RetryHandlerOption(3),
	)

	return &Router{
		server:  &gost.Server{Listener: ln},
		handler: handler,
	}, nil
}

func parseForwardChain(opts *Options) (*gost.Chain, error) {
	var protocol string
	if opts.tlsEnabled {
		switch opts.mode {
		case "ws":
			protocol = "wss"
		case "mws":
			protocol = "mwss"
		}
	}

	if len(protocol) == 0 {
		protocol = opts.mode
	}

	url := fmt.Sprintf("%s://%s:%d?path=%s", protocol, fixAddr(opts.remoteAddr), opts.remotePort, opts.path)
	node, err := gost.ParseNode(url)
	if err != nil {
		return nil, err
	}

	var tr gost.Transporter
	wsOpts := &WSOptions{node: &node}
	wsOpts.Path = opts.path
	wsOpts.Fastopen = opts.fastopen
	mux := uint16(opts.mux)
	if mux < 1 {
		mux = 1
	}
	wsOpts.muxSessions = mux
	if !opts.nocomp {
		wsOpts.EnableCompression = true
	}
	switch node.Transport {
	case "ws":
		tr = WSTransporter(wsOpts)
	case "mws":
		tr = MWSTransporter(wsOpts)
	case "wss":
		tr = WSSTransporter(wsOpts)
	case "mwss":
		tr = MWSSTransporter(wsOpts)
	default:
		return nil, newError("unsupported mode:", opts.mode)
	}

	node.Protocol = "http" // default protocol is HTTP
	connector := gost.HTTPConnector(node.User)

	var serverName string
	if len(opts.hostname) >= 0 {
		serverName = opts.hostname
	} else {
		serverName, _, _ = net.SplitHostPort(node.Addr)
	}

	tlsCfg := &tls.Config{
		ServerName: serverName,
	}

	node.HandshakeOptions = []gost.HandshakeOption{
		gost.AddrHandshakeOption(node.Addr),
		gost.HostHandshakeOption(node.Host),
		gost.UserHandshakeOption(node.User),
		gost.TLSConfigHandshakeOption(tlsCfg),
	}
	node.Client = &gost.Client{
		Connector:   connector,
		Transporter: tr,
	}

	chain := gost.NewChain()
	chain.AddNode(node)

	return chain, nil
}

func configServer(opts *Options) (*Router, error) {
	chain, err := parseServerChain(opts)
	if err != nil {
		return nil, err
	}

	var protocol string
	if opts.tlsEnabled {
		switch opts.mode {
		case "ws":
			protocol = "wss"
		case "mws":
			protocol = "mwss"
		}
	}

	if len(protocol) == 0 {
		protocol = opts.mode
	}

	url := fmt.Sprintf("%s://%s:%d?path=%s", protocol, fixAddr(opts.localAddr), opts.localPort, opts.path)
	node, err := gost.ParseNode(url)
	if err != nil {
		return nil, err
	}

	var tlsCfg *tls.Config
	if opts.tlsEnabled {
		if opts.cert == "" || opts.key == "" {
			return nil, newError("No cert file or key file specified")
		}

		cert, err := tls.LoadX509KeyPair(opts.cert, opts.key)
		if err != nil {
			return nil, err
		}
		tlsCfg = &tls.Config{Certificates: []tls.Certificate{cert}}
	}

	var ln gost.Listener
	wsOpts := &gost.WSOptions{Path: opts.path}
	if !opts.nocomp {
		wsOpts.EnableCompression = true
	}
	switch node.Transport {
	case "ws":
		ln, err = gost.WSListener(node.Addr, wsOpts)
	case "mws":
		ln, err = gost.MWSListener(node.Addr, wsOpts)
	case "wss":
		ln, err = gost.WSSListener(node.Addr, tlsCfg, wsOpts)
	case "mwss":
		ln, err = gost.MWSSListener(node.Addr, tlsCfg, wsOpts)
	default:
		return nil, newError("unsupported mode:", opts.mode)
	}

	handler := gost.HTTPHandler()
	handler.Init(
		gost.AddrHandlerOption(ln.Addr().String()),
		gost.ChainHandlerOption(chain),
	)

	return &Router{
		server:  &gost.Server{Listener: ln},
		handler: handler,
	}, nil
}

func parseServerChain(opts *Options) (*gost.Chain, error) {
	var url string
	var tr gost.Transporter
	if strings.HasPrefix(opts.remoteAddr, "unix://") {
		url = opts.remoteAddr
		tr = UnixTransporter(opts.remoteAddr[7:])
	} else {
		url = fmt.Sprintf("tcp://%s:%d", fixAddr(opts.remoteAddr), opts.remotePort)
		tr = gost.TCPTransporter()
	}

	node, err := gost.ParseNode(url)
	if err != nil {
		return nil, err
	}

	node.Protocol = "forward"
	connector := gost.ForwardConnector()

	node.Client = &gost.Client{
		Connector:   connector,
		Transporter: tr,
	}

	chain := gost.NewChain()
	chain.AddNode(node)

	return chain, nil
}

func fixAddr(addr string) string {
	size := len(addr)
	if size == 0 {
		return addr
	}

	if strings.Contains(addr, ":") && addr[0] != '[' && addr[size-1] != ']' {
		return "[" + addr + "]"
	}

	return addr
}
