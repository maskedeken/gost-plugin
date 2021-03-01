package protocol

import (
	context "context"
	"net"
	"time"

	"github.com/maskedeken/gost-plugin/args"
	C "github.com/maskedeken/gost-plugin/constant"
	"github.com/maskedeken/gost-plugin/gost"
	"github.com/maskedeken/gost-plugin/log"
	"github.com/maskedeken/gost-plugin/registry"
	grpc "google.golang.org/grpc"
	"google.golang.org/grpc/backoff"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/peer"
)

type GunListener struct {
	*TCPListener
	server *grpc.Server
}

// Close implements gost.Listener.Close()
func (l *GunListener) Close() error {
	return l.listener.Close()
}

// AcceptConn implements gost.Listener.AcceptConn()
func (l *GunListener) AcceptConn() (conn net.Conn, err error) {
	conn = <-l.connChan
	return
}

// Serve implements gost.Listener.Serve()
func (l *GunListener) Serve(ctx context.Context) error {
	return l.server.Serve(l.listener)
}

// Tun implements GunServiceServer.Tun()
func (l *GunListener) Tun(srv GunService_TunServer) error {
	var remote net.Addr
	pr, ok := peer.FromContext(srv.Context())
	if ok {
		remote = pr.Addr
	}
	conn := newGunConnection(srv, l.listener.Addr(), remote)

	select {
	case l.connChan <- conn:
	default:
		log.Warnln("connection queue is full")
		conn.Close()
	}

	<-conn.Done()
	return nil
}

// NewGunListener is the constructor for GunListener
func NewGunListener(ctx context.Context) (gost.Listener, error) {
	inner, err := NewTCPListener(ctx)
	if err != nil {
		return nil, err
	}

	tlsConfig, err := buildServerTLSConfig(ctx)
	if err != nil {
		return nil, err
	}

	server := grpc.NewServer(grpc.Creds(credentials.NewTLS(tlsConfig)))
	l := &GunListener{
		TCPListener: inner.(*TCPListener),
		server:      server,
	}
	RegisterGunServiceServer(server, l)
	return l, nil
}

type GunTransporter struct {
	*TCPTransporter
	client GunServiceClient
}

// DialConn implements gost.Transporter.DialConn()
func (t *GunTransporter) DialConn() (net.Conn, error) {
	// connect rpc
	tun, err := t.client.Tun(context.Background())
	if err != nil {
		return nil, err
	}

	var remote net.Addr
	pr, ok := peer.FromContext(tun.Context())
	if ok {
		remote = pr.Addr
	}
	return newGunConnection(tun, nil, remote), nil
}

// NewGunTransporter is the constructor for GunTransporter
func NewGunTransporter(ctx context.Context) (gost.Transporter, error) {
	inner, err := NewTCPTransporter(ctx)
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

	client := NewGunServiceClient(conn)
	return &GunTransporter{
		TCPTransporter: inner.(*TCPTransporter),
		client:         client,
	}, nil
}

type gunService interface {
	Send(*Hunk) error
	Recv() (*Hunk, error)
}

type gunConnection struct {
	gunService
	local  net.Addr
	remote net.Addr
	rb     []byte
	done   chan struct{}
}

func newGunConnection(service gunService, local net.Addr, remote net.Addr) *gunConnection {
	if local == nil {
		local = &net.TCPAddr{
			IP:   []byte{0, 0, 0, 0},
			Port: 0,
		}
	}

	if remote == nil {
		remote = &net.TCPAddr{
			IP:   []byte{0, 0, 0, 0},
			Port: 0,
		}
	}
	return &gunConnection{
		gunService: service,
		local:      local,
		remote:     remote,
		done:       make(chan struct{}),
	}
}

// Read implements net.Conn.Read().
func (c *gunConnection) Read(b []byte) (int, error) {
	if len(c.rb) == 0 {
		hunk, err := c.gunService.Recv()
		if err != nil {
			return 0, err
		}

		c.rb = hunk.Data
	}

	n := copy(b, c.rb)
	c.rb = c.rb[n:]
	return n, nil
}

// Write implements net.Conn.Write().
func (c *gunConnection) Write(b []byte) (int, error) {
	err := c.gunService.Send(&Hunk{Data: b})
	if err != nil {
		return 0, err
	}

	return len(b), nil
}

// Close implements net.Conn.Close().
func (c *gunConnection) Close() error {
	close(c.done)
	return nil
}

// LocalAddr implements net.Conn.LocalAddr().
func (c *gunConnection) LocalAddr() net.Addr {
	return c.local
}

// RemoteAddr implements net.Conn.RemoteAddr().
func (c *gunConnection) RemoteAddr() net.Addr {
	return c.remote
}

// SetDeadline implements net.Conn.SetDeadline().
func (c *gunConnection) SetDeadline(t time.Time) error {
	return nil
}

// SetReadDeadline implements net.Conn.SetReadDeadline().
func (c *gunConnection) SetReadDeadline(t time.Time) error {
	return nil
}

// SetWriteDeadline implements net.Conn.SetWriteDeadline().
func (c *gunConnection) SetWriteDeadline(t time.Time) error {
	return nil
}

func (c *gunConnection) Done() <-chan struct{} {
	return c.done
}

func init() {
	registry.RegisterListener("grpc", NewGunListener)
	registry.RegisterTransporter("grpc", NewGunTransporter)

	registry.RegisterListener("gun", NewGunListener)
	registry.RegisterTransporter("gun", NewGunTransporter)
}
