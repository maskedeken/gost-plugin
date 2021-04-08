package http

import (
	"io"
	"net"
	"net/http"
	"time"
)

type HttpConnection struct {
	reader io.Reader
	writer io.Writer
	closer io.Closer
	local  net.Addr
	remote net.Addr
	done   chan struct{}
}

func (c *HttpConnection) isClosed() bool {
	select {
	case <-c.done:
		return true
	default:
		return false
	}
}

// Read implements net.Conn.Read().
func (c *HttpConnection) Read(b []byte) (int, error) {
	return c.reader.Read(b)
}

// Write implements net.Conn.Write().
func (c *HttpConnection) Write(b []byte) (int, error) {
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
func (c *HttpConnection) Close() error {
	err := c.closer.Close()
	close(c.done)
	return err
}

// LocalAddr implements net.Conn.LocalAddr().
func (c *HttpConnection) LocalAddr() net.Addr {
	return c.local
}

// RemoteAddr implements net.Conn.RemoteAddr().
func (c *HttpConnection) RemoteAddr() net.Addr {
	return c.remote
}

// SetDeadline implements net.Conn.SetDeadline().
func (c *HttpConnection) SetDeadline(t time.Time) error {
	return nil
}

// SetReadDeadline implements net.Conn.SetReadDeadline().
func (c *HttpConnection) SetReadDeadline(t time.Time) error {
	return nil
}

// SetWriteDeadline implements net.Conn.SetWriteDeadline().
func (c *HttpConnection) SetWriteDeadline(t time.Time) error {
	return nil
}

func (c *HttpConnection) Done() <-chan struct{} {
	return c.done
}

func NewHttpConnection(reader io.ReadCloser, writer io.Writer, closer io.Closer, localAddr net.Addr, remoteAddr net.Addr) *HttpConnection {
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

	return &HttpConnection{
		local:  localAddr,
		remote: remoteAddr,
		reader: reader,
		writer: writer,
		closer: closer,
		done:   make(chan struct{}),
	}
}
