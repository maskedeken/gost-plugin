package gost

import (
	"context"
	"io"
	"net"
)

type Listener interface {
	io.Closer
	Serve(context.Context) error
	AcceptConn() (net.Conn, error)
}
