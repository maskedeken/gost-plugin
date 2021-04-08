package readv

import (
	"io"
	"net"
	"syscall"

	xtls "github.com/xtls/go"
)

type readVConn struct {
	*xtls.Conn
	rawConn syscall.RawConn
}

func (c *readVConn) Read(b []byte) (int, error) {
	if c.rawConn != nil && c.DirectIn {
		var nBytes int
		var err error
		err = c.rawConn.Read(func(fd uintptr) (done bool) {
			nBytes = ReadRaw(fd, b)
			return nBytes > -1
		})

		if err != nil {
			return 0, err
		}

		if nBytes == 0 {
			return 0, io.EOF
		}

		return nBytes, err
	}

	return c.Conn.Read(b)
}

func (c *readVConn) GetXTLSConn() *xtls.Conn {
	return c.Conn
}

func NewReadVConn(xConn *xtls.Conn, rawConn syscall.RawConn) net.Conn {
	return &readVConn{
		Conn:    xConn,
		rawConn: rawConn,
	}
}
