package server

import (
	"context"
	"crypto/tls"
	"net"
	"net/http"
	"time"

	"github.com/maskedeken/gost-plugin/args"
	C "github.com/maskedeken/gost-plugin/constant"
	"github.com/maskedeken/gost-plugin/gost"
	H "github.com/maskedeken/gost-plugin/gost/protocol/http"
	"github.com/maskedeken/gost-plugin/gost/proxy"
	"github.com/maskedeken/gost-plugin/log"
	"github.com/maskedeken/gost-plugin/registry"
)

// H2Listener is Listener which handles http/2
type H2Listener struct {
	*proxy.TCPListener
	server *http.Server
	addr   net.Addr
	path   string
}

// Serve implements gost.Listener.Serve()
func (l *H2Listener) Serve(ctx context.Context) error {
	return l.server.Serve(l.Listener)
}

// AcceptConn implements gost.Listener.AcceptConn()
func (l *H2Listener) AcceptConn() (net.Conn, error) {
	return <-l.ConnChan, nil
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
	conn := H.NewHttpConnection(r.Body, w, r.Body, l.addr, remoteAddr)

	select {
	case l.ConnChan <- conn:
	default:
		log.Warnln("connection queue is full")
		conn.Close()
	}

	<-conn.Done()
}

// NewH2Listener is the constructor for H2Listener
func NewH2Listener(ctx context.Context) (gost.Listener, error) {
	inner, err := proxy.NewTCPListener(ctx)
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
		TCPListener: inner.(*proxy.TCPListener),
		addr:        lAddr,
		path:        options.Path,
	}
	l.Listener = tls.NewListener(l.Listener, tlsConfig) // turn net.Listener into tls.Listener

	mux := http.NewServeMux()
	mux.Handle(options.Path, http.HandlerFunc(l.serveHTTP))
	l.server = &http.Server{
		Addr:              lAddrStr,
		Handler:           mux,
		ReadHeaderTimeout: 30 * time.Second,
	}

	return l, nil
}

func init() {
	registry.RegisterListener("h2", NewH2Listener)
}
