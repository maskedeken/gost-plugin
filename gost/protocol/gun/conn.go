package gun

import (
	context "context"
	"net"
	"time"

	"google.golang.org/grpc/peer"
)

type GunService interface {
	Context() context.Context
	Send(*Hunk) error
	Recv() (*Hunk, error)
}

type GunConnection struct {
	gunService GunService
	local      net.Addr
	remote     net.Addr
	rb         []byte
	done       chan struct{}
}

func NewGunConnection(service GunService, local net.Addr) *GunConnection {
	var remote net.Addr
	pr, ok := peer.FromContext(service.Context())
	if ok {
		remote = pr.Addr
	} else {
		remote = &net.TCPAddr{
			IP:   []byte{0, 0, 0, 0},
			Port: 0,
		}
	}

	if local == nil {
		local = &net.TCPAddr{
			IP:   []byte{0, 0, 0, 0},
			Port: 0,
		}
	}

	return &GunConnection{
		gunService: service,
		local:      local,
		remote:     remote,
		done:       make(chan struct{}),
	}
}

// Read implements net.Conn.Read().
func (c *GunConnection) Read(b []byte) (int, error) {
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
func (c *GunConnection) Write(b []byte) (int, error) {
	err := c.gunService.Send(&Hunk{Data: b})
	if err != nil {
		return 0, err
	}

	return len(b), nil
}

// Close implements net.Conn.Close().
func (c *GunConnection) Close() error {
	close(c.done)
	return nil
}

// LocalAddr implements net.Conn.LocalAddr().
func (c *GunConnection) LocalAddr() net.Addr {
	return c.local
}

// RemoteAddr implements net.Conn.RemoteAddr().
func (c *GunConnection) RemoteAddr() net.Addr {
	return c.remote
}

// SetDeadline implements net.Conn.SetDeadline().
func (c *GunConnection) SetDeadline(t time.Time) error {
	return nil
}

// SetReadDeadline implements net.Conn.SetReadDeadline().
func (c *GunConnection) SetReadDeadline(t time.Time) error {
	return nil
}

// SetWriteDeadline implements net.Conn.SetWriteDeadline().
func (c *GunConnection) SetWriteDeadline(t time.Time) error {
	return nil
}

func (c *GunConnection) Done() <-chan struct{} {
	return c.done
}
