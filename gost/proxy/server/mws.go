package server

import (
	"context"
	"crypto/tls"
	"encoding/base64"
	"net/http"

	"github.com/maskedeken/gost-plugin/args"
	C "github.com/maskedeken/gost-plugin/constant"
	"github.com/maskedeken/gost-plugin/gost"
	"github.com/maskedeken/gost-plugin/gost/proxy"
	"github.com/maskedeken/gost-plugin/log"
	"github.com/maskedeken/gost-plugin/registry"
)

// MWSListener is Listener which handles multiplex websocket protocol
type MWSListener struct {
	*WSListener
}

// Upgrade turns net.Conn into websocket conn
func (l *MWSListener) Upgrade(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != l.path {
		w.WriteHeader(http.StatusNotFound)
		return
	}

	conn, err := l.upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Errorf("failed to upgrade to websocket: %s", err)
		return
	}

	var ed []byte
	if str := r.Header.Get("Sec-WebSocket-Protocol"); str != "" {
		if ed, err = base64.StdEncoding.DecodeString(str); err != nil {
			log.Errorf("failed to decode the early payload: %s", err)
			return
		}
	}

	go proxy.ServeMux(websocketServerConn(conn, ed), l.ConnChan)
}

// NewMWSListener is constructor for MWSListener
func NewMWSListener(ctx context.Context) (gost.Listener, error) {
	inner, err := NewWSListener(ctx)
	if err != nil {
		return nil, err
	}

	l := &MWSListener{inner.(*WSListener)}
	options := ctx.Value(C.OPTIONS).(*args.Options)
	mux := http.NewServeMux()
	mux.Handle(options.Path, http.HandlerFunc(l.Upgrade))
	l.server.Handler = mux // replace handler
	return l, nil
}

// MWSSListener is Listener which handles multiplex websocket over tls
type MWSSListener struct {
	*MWSListener
}

// NewMWSSListener is constructor for MWSSListener
func NewMWSSListener(ctx context.Context) (gost.Listener, error) {
	inner, err := NewMWSListener(ctx)
	if err != nil {
		return nil, err
	}

	tlsConfig, err := buildServerTLSConfig(ctx)
	if err != nil {
		return nil, err
	}

	l := &MWSSListener{inner.(*MWSListener)}
	l.Listener = tls.NewListener(l.Listener, tlsConfig) // turn listener into tls.Listener
	return l, nil
}

func init() {
	registry.RegisterListener("mws", NewMWSListener)
	registry.RegisterListener("mwss", NewMWSSListener)
}
