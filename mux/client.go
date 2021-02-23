package mux

import (
	"net"

	"github.com/xtaci/smux"
)

type client interface {
	OpenNewStream() (net.Conn, error)
	Close() error
	IsClosed() bool
	NumStreams() int
}

type smuxClient struct {
	*smux.Session
}

func (c *smuxClient) OpenNewStream() (net.Conn, error) {
	return c.OpenStream()
}
