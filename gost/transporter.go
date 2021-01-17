package gost

import (
	"net"
)

type Transporter interface {
	DialConn() (net.Conn, error)
}
