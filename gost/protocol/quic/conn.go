package quic

import (
	"net"
	"time"

	"github.com/lucas-clemente/quic-go"
)

type interConn struct {
	stream quic.Stream
	local  net.Addr
	remote net.Addr
}

func (c *interConn) Read(b []byte) (int, error) {
	return c.stream.Read(b)
}

func (c *interConn) Write(b []byte) (int, error) {
	return c.stream.Write(b)
}

func (c *interConn) Close() error {
	return c.stream.Close()
}

func (c *interConn) LocalAddr() net.Addr {
	return c.local
}

func (c *interConn) RemoteAddr() net.Addr {
	return c.remote
}

func (c *interConn) SetDeadline(t time.Time) error {
	return c.stream.SetDeadline(t)
}

func (c *interConn) SetReadDeadline(t time.Time) error {
	return c.stream.SetReadDeadline(t)
}

func (c *interConn) SetWriteDeadline(t time.Time) error {
	return c.stream.SetWriteDeadline(t)
}

func NewQUICConn(stream quic.Stream, local net.Addr, remote net.Addr) net.Conn {
	return &interConn{
		stream: stream,
		local:  local,
		remote: remote,
	}
}
