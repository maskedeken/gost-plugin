package server

import (
	"context"
	"net"

	"github.com/maskedeken/gost-plugin/args"
	C "github.com/maskedeken/gost-plugin/constant"
	"github.com/maskedeken/gost-plugin/gost"
	G "github.com/maskedeken/gost-plugin/gost/protocol/gun"
	"github.com/maskedeken/gost-plugin/gost/proxy"
	"github.com/maskedeken/gost-plugin/log"
	"github.com/maskedeken/gost-plugin/registry"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
)

type GunListener struct {
	*proxy.TCPListener
	server *grpc.Server
}

// Close implements gost.Listener.Close()
func (l *GunListener) Close() error {
	return l.Listener.Close()
}

// AcceptConn implements gost.Listener.AcceptConn()
func (l *GunListener) AcceptConn() (conn net.Conn, err error) {
	conn = <-l.ConnChan
	return
}

// Serve implements gost.Listener.Serve()
func (l *GunListener) Serve(ctx context.Context) error {
	return l.server.Serve(l.Listener)
}

// Tun implements GunServiceServer.Tun()
func (l *GunListener) Tun(srv G.GunService_TunServer) error {
	conn := G.NewGunConnection(srv, l.Listener.Addr())

	select {
	case l.ConnChan <- conn:
	default:
		log.Warnln("connection queue is full")
		conn.Close()
	}

	<-conn.Done()
	return nil
}

// NewGunListener is the constructor for GunListener
func NewGunListener(ctx context.Context) (gost.Listener, error) {
	inner, err := proxy.NewTCPListener(ctx)
	if err != nil {
		return nil, err
	}

	tlsConfig, err := buildServerTLSConfig(ctx)
	if err != nil {
		return nil, err
	}

	options := ctx.Value(C.OPTIONS).(*args.Options)
	server := grpc.NewServer(grpc.Creds(credentials.NewTLS(tlsConfig)))
	l := &GunListener{
		TCPListener: inner.(*proxy.TCPListener),
		server:      server,
	}

	desc := G.ServerDesc(options.ServiceName)
	server.RegisterService(&desc, l)
	return l, nil
}

func init() {
	registry.RegisterListener("grpc", NewGunListener)
	registry.RegisterListener("gun", NewGunListener)
}
