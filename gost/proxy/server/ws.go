package server

import (
	"context"
	"crypto/tls"
	"encoding/base64"
	"net"
	"net/http"
	"time"

	"github.com/gorilla/websocket"
	"github.com/maskedeken/gost-plugin/args"
	C "github.com/maskedeken/gost-plugin/constant"
	"github.com/maskedeken/gost-plugin/gost"
	ws "github.com/maskedeken/gost-plugin/gost/protocol/websocket"
	"github.com/maskedeken/gost-plugin/gost/proxy"
	"github.com/maskedeken/gost-plugin/log"
	"github.com/maskedeken/gost-plugin/registry"
)

// WSListener is Listener which handles websocket protocol
type WSListener struct {
	*proxy.TCPListener
	upgrader *websocket.Upgrader
	server   *http.Server
	path     string
}

// Close implements gost.Listener.Close()
func (l *WSListener) Close() error {
	return l.Listener.Close()
}

// Serve implements gost.Listener.Serve()
func (l *WSListener) Serve(ctx context.Context) error {
	return l.server.Serve(l.Listener)
}

// AcceptConn implements gost.Listener.AcceptConn()
func (l *WSListener) AcceptConn() (net.Conn, error) {
	return <-l.ConnChan, nil
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
	case l.ConnChan <- websocketServerConn(conn, ed):
	default:
		log.Warnln("connection queue is full")
		conn.Close()
	}
}

// NewWSListener is the constructor for WSListener
func NewWSListener(ctx context.Context) (gost.Listener, error) {
	inner, err := proxy.NewTCPListener(ctx)
	if err != nil {
		return nil, err
	}

	options := ctx.Value(C.OPTIONS).(*args.Options)
	l := &WSListener{
		TCPListener: inner.(*proxy.TCPListener),
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
	l.Listener = tls.NewListener(l.Listener, tlsConfig) // turn listener into tls.Listener
	return l, nil
}

func websocketServerConn(conn *websocket.Conn, ed []byte) net.Conn {
	// conn.EnableWriteCompression(true)
	return ws.NewWebsocketConn(conn, ed)
}

func init() {
	registry.RegisterListener("ws", NewWSListener)
	registry.RegisterListener("wss", NewWSSListener)
}
