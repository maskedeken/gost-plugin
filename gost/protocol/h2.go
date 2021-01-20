package protocol

import (
	"context"
	"crypto/tls"
	"errors"
	"io"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"time"

	"github.com/maskedeken/gost-plugin/args"
	C "github.com/maskedeken/gost-plugin/constant"
	"github.com/maskedeken/gost-plugin/gost"
	"github.com/maskedeken/gost-plugin/registry"
	"golang.org/x/net/http2"
)

// H2Listener is Listener which handles http/2
type H2Listener struct {
	*TCPListener
	server *http.Server
	addr   net.Addr
	path   string
}

// Serve implements gost.Listener.Serve()
func (l *H2Listener) Serve(ctx context.Context) error {
	return l.server.Serve(l.listener)
}

// AcceptConn implements gost.Listener.AcceptConn()
func (l *H2Listener) AcceptConn() (net.Conn, error) {
	return <-l.connChan, nil
}

func (l *H2Listener) serveHTTP(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != l.path {
		w.WriteHeader(http.StatusNotFound)
		return
	}

	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(http.StatusOK)
	if f, ok := w.(http.Flusher); ok {
		f.Flush()
	}

	remoteAddr, _ := net.ResolveTCPAddr("tcp", r.RemoteAddr)
	conn := newHttpConnection(r.Body, w, r.Body, l.addr, remoteAddr)

	l.connChan <- conn
	<-conn.Done()
}

// NewH2Listener is the constructor for H2Listener
func NewH2Listener(ctx context.Context) (gost.Listener, error) {
	inner, err := NewTCPListener(ctx)
	if err != nil {
		return nil, err
	}

	tlsConfig, err := buildServerTLSConfig(ctx)
	if err != nil {
		return nil, err
	}
	tlsConfig.NextProtos = []string{"h2"}

	options := ctx.Value(C.OPTIONS).(*args.Options)
	lAddrStr := options.GetLocalAddr()
	lAddr, _ := net.ResolveTCPAddr("tcp", lAddrStr)
	l := &H2Listener{
		TCPListener: inner.(*TCPListener),
		addr:        lAddr,
		path:        options.Path,
	}
	ln := tls.NewListener(l.listener, tlsConfig)
	l.listener = ln // turn net.Listener into tls.Listener

	mux := http.NewServeMux()
	mux.Handle(options.Path, http.HandlerFunc(l.serveHTTP))
	l.server = &http.Server{
		Addr:              lAddrStr,
		Handler:           mux,
		ReadHeaderTimeout: 30 * time.Second,
	}

	return l, nil
}

// H2Transporter is Transporter which handles http/2
type H2Transporter struct {
	*TCPTransporter
	ctx    context.Context
	client *http.Client
}

// DialConn implements gost.Transporter.DialConn()
func (t *H2Transporter) DialConn() (net.Conn, error) {
	options := t.ctx.Value(C.OPTIONS).(*args.Options)
	reader, writer := io.Pipe()
	request := &http.Request{
		Method: "PUT",
		Body:   reader,
		URL: &url.URL{
			Scheme: "https",
			Host:   options.GetRemoteAddr(),
			Path:   options.Path,
		},
		Proto:      "HTTP/2",
		ProtoMajor: 2,
		ProtoMinor: 0,
		Header:     make(http.Header),
	}
	if options.Hostname != "" {
		request.Host = options.Hostname
	}
	// Disable any compression method from server.
	request.Header.Set("Accept-Encoding", "identity")

	response, err := t.client.Do(request)
	if err != nil {
		return nil, err
	}
	if response.StatusCode != 200 {
		return nil, errors.New("unexpected status " + strconv.Itoa(response.StatusCode))
	}

	return newHttpConnection(response.Body, writer, chainedClosable{reader, writer, response.Body}, nil, nil), nil

}

// NewH2Transporter is the constructor for H2Transporter
func NewH2Transporter(ctx context.Context) (gost.Transporter, error) {
	inner, err := NewTCPTransporter(ctx)
	if err != nil {
		return nil, err
	}

	tlsConfig := buildClientTLSConfig(ctx)
	tlsConfig.NextProtos = []string{"h2"}
	transport := &http2.Transport{
		DialTLS: func(network string, addr string, tlsConfig *tls.Config) (net.Conn, error) {
			pconn, err := inner.DialConn()
			if err != nil {
				return nil, err
			}

			cn := tls.Client(pconn, tlsConfig)
			if err := cn.Handshake(); err != nil {
				return nil, err
			}
			state := cn.ConnectionState()
			if p := state.NegotiatedProtocol; p != http2.NextProtoTLS {
				return nil, errors.New("http2: unexpected ALPN protocol " + p + "; want q" + http2.NextProtoTLS)
			}
			if !state.NegotiatedProtocolIsMutual {
				return nil, errors.New("http2: could not negotiate protocol mutually")
			}
			return cn, nil
		},
		TLSClientConfig: tlsConfig,
	}

	client := &http.Client{
		Transport: transport,
	}

	return &H2Transporter{
		TCPTransporter: inner.(*TCPTransporter),
		ctx:            ctx,
		client:         client,
	}, nil
}

type chainedClosable []io.Closer

// Close implements io.Closer.Close().
func (cc chainedClosable) Close() error {
	for _, c := range cc {
		_ = c.Close()
	}
	return nil
}

type httpConnection struct {
	reader io.Reader
	writer io.Writer
	closer io.Closer
	local  net.Addr
	remote net.Addr
	done   chan struct{}
}

func (c *httpConnection) isClosed() bool {
	select {
	case <-c.done:
		return true
	default:
		return false
	}
}

// Read implements net.Conn.Read().
func (c *httpConnection) Read(b []byte) (int, error) {
	return c.reader.Read(b)
}

// Write implements net.Conn.Write().
func (c *httpConnection) Write(b []byte) (int, error) {
	if c.isClosed() {
		return 0, io.ErrClosedPipe
	}

	n, err := c.writer.Write(b)
	if err != nil {
		return 0, err
	}

	if f, ok := c.writer.(http.Flusher); ok {
		f.Flush()
	}

	return n, err
}

// Close implements net.Conn.Close().
func (c *httpConnection) Close() error {
	err := c.closer.Close()
	close(c.done)
	return err
}

// LocalAddr implements net.Conn.LocalAddr().
func (c *httpConnection) LocalAddr() net.Addr {
	return c.local
}

// RemoteAddr implements net.Conn.RemoteAddr().
func (c *httpConnection) RemoteAddr() net.Addr {
	return c.remote
}

// SetDeadline implements net.Conn.SetDeadline().
func (c *httpConnection) SetDeadline(t time.Time) error {
	return nil
}

// SetReadDeadline implements net.Conn.SetReadDeadline().
func (c *httpConnection) SetReadDeadline(t time.Time) error {
	return nil
}

// SetWriteDeadline implements net.Conn.SetWriteDeadline().
func (c *httpConnection) SetWriteDeadline(t time.Time) error {
	return nil
}

func (c *httpConnection) Done() <-chan struct{} {
	return c.done
}

func newHttpConnection(reader io.ReadCloser, writer io.Writer, closer io.Closer, localAddr net.Addr, remoteAddr net.Addr) *httpConnection {
	if localAddr == nil {
		localAddr = &net.TCPAddr{
			IP:   []byte{0, 0, 0, 0},
			Port: 0,
		}
	}

	if remoteAddr == nil {
		remoteAddr = &net.TCPAddr{
			IP:   []byte{0, 0, 0, 0},
			Port: 0,
		}
	}

	return &httpConnection{
		local:  localAddr,
		remote: remoteAddr,
		reader: reader,
		writer: writer,
		closer: closer,
		done:   make(chan struct{}),
	}
}

func init() {
	registry.RegisterListener("h2", NewH2Listener)
	registry.RegisterTransporter("h2", NewH2Transporter)
}
