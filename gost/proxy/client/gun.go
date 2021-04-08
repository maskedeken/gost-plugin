package client

import (
	"context"
	"net"
	"time"

	"github.com/maskedeken/gost-plugin/args"
	C "github.com/maskedeken/gost-plugin/constant"
	"github.com/maskedeken/gost-plugin/gost"
	G "github.com/maskedeken/gost-plugin/gost/protocol/gun"
	"github.com/maskedeken/gost-plugin/gost/proxy"
	"github.com/maskedeken/gost-plugin/registry"
	"google.golang.org/grpc"
	"google.golang.org/grpc/backoff"
	"google.golang.org/grpc/credentials"
)

type GunTransporter struct {
	*proxy.TCPTransporter
	client G.GunServiceClientX
}

// DialConn implements gost.Transporter.DialConn()
func (t *GunTransporter) DialConn() (net.Conn, error) {
	// connect rpc
	options := t.Context.Value(C.OPTIONS).(*args.Options)
	tun, err := t.client.TunCustomName(context.Background(), options.ServiceName)
	if err != nil {
		return nil, err
	}

	return G.NewGunConnection(tun, nil), nil
}

// NewGunTransporter is the constructor for GunTransporter
func NewGunTransporter(ctx context.Context) (gost.Transporter, error) {
	inner, err := proxy.NewTCPTransporter(ctx)
	if err != nil {
		return nil, err
	}

	options := ctx.Value(C.OPTIONS).(*args.Options)

	var dialOption grpc.DialOption
	tlsConfig := buildClientTLSConfig(ctx)
	dialOption = grpc.WithTransportCredentials(credentials.NewTLS(tlsConfig))

	// dial
	conn, err := grpc.Dial(
		options.GetRemoteAddr(),
		dialOption,
		grpc.WithConnectParams(grpc.ConnectParams{
			Backoff: backoff.Config{
				BaseDelay:  500 * time.Millisecond,
				Multiplier: 1.5,
				Jitter:     0.2,
				MaxDelay:   19 * time.Millisecond,
			},
			MinConnectTimeout: 5 * time.Second,
		}),
		grpc.WithDialer(func(string, time.Duration) (net.Conn, error) {
			return inner.DialConn()
		}),
	)
	if err != nil {
		return nil, err
	}

	client := G.NewGunServiceClientX(conn)
	return &GunTransporter{
		TCPTransporter: inner.(*proxy.TCPTransporter),
		client:         client,
	}, nil
}

func init() {
	registry.RegisterTransporter("grpc", NewGunTransporter)
	registry.RegisterTransporter("gun", NewGunTransporter)
}
