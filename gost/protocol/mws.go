package protocol

import (
	"context"
	"crypto/tls"
	"net"
	"net/http"

	"github.com/maskedeken/gost-plugin/args"
	C "github.com/maskedeken/gost-plugin/constant"
	"github.com/maskedeken/gost-plugin/gost"
	"github.com/maskedeken/gost-plugin/log"
	"github.com/maskedeken/gost-plugin/mux"
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

	go serveMux(websocketServerConn(conn), l.connChan)
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
	ln := l.listener
	l.listener = tls.NewListener(ln, tlsConfig) // turn listener into tls.Listener
	return l, nil
}

// MWSTransporter is Transporter which handles multiplex websocket protocol
type MWSTransporter struct {
	*WSTransporter
	pool *mux.MuxPool
}

// DialConn implements gost.Transporter.DialConn()
func (t *MWSTransporter) DialConn() (net.Conn, error) {
	return t.pool.DialMux(t.WSTransporter.DialConn)
}

// NewMWSTransporter is constructor for MWSTransporter
func NewMWSTransporter(ctx context.Context) (gost.Transporter, error) {
	inner, err := NewWSTransporter(ctx)
	if err != nil {
		return nil, err
	}

	return &MWSTransporter{WSTransporter: inner.(*WSTransporter),
		pool: mux.NewMuxPool(ctx),
	}, nil
}

// MWSSTransporter is Transporter which handles multiplex websocket over tls
type MWSSTransporter struct {
	*WSSTransporter
	pool *mux.MuxPool
}

// DialConn implements gost.Transporter.DialConn()
func (t *MWSSTransporter) DialConn() (net.Conn, error) {
	return t.pool.DialMux(t.WSSTransporter.DialConn)
}

// NewMWSSTransporter is constructor for MWSSTransporter
func NewMWSSTransporter(ctx context.Context) (gost.Transporter, error) {
	inner, err := NewWSSTransporter(ctx)
	if err != nil {
		return nil, err
	}

	return &MWSSTransporter{WSSTransporter: inner.(*WSSTransporter),
		pool: mux.NewMuxPool(ctx),
	}, nil
}

func init() {
	registry.RegisterListener("mws", NewMWSListener)
	registry.RegisterTransporter("mws", NewMWSTransporter)

	registry.RegisterListener("mwss", NewMWSSListener)
	registry.RegisterTransporter("mwss", NewMWSSTransporter)
}
