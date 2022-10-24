package client

import (
	"context"
	"net"

	"google.golang.org/grpc/credentials"

	"crypto/tls"
)

// grpcTLS is the credentials required for authenticating a connection using TLS.
type grpcTLS struct {
	config      *tls.Config
	fingerprint string
}

func (c grpcTLS) Info() credentials.ProtocolInfo {
	return credentials.ProtocolInfo{
		SecurityProtocol: "tls",
		SecurityVersion:  "1.2",
		ServerName:       c.config.ServerName,
	}
}

func (c *grpcTLS) ClientHandshake(ctx context.Context, authority string, rawConn net.Conn) (_ net.Conn, _ credentials.AuthInfo, err error) {
	// use local cfg to avoid clobbering ServerName if using multiple endpoints
	cfg := c.config.Clone()
	if cfg.ServerName == "" {
		serverName, _, err := net.SplitHostPort(authority)
		if err != nil {
			// If the authority had no host port or if the authority cannot be parsed, use it as-is.
			serverName = authority
		}
		cfg.ServerName = serverName
	}
	cn := newClientTLSConn(rawConn, c.config, c.fingerprint)
	errChannel := make(chan error, 1)
	go func() {
		errChannel <- cn.Handshake()
		close(errChannel)
	}()
	select {
	case err := <-errChannel:
		if err != nil {
			cn.Close()
			return nil, nil, err
		}
	case <-ctx.Done():
		cn.Close()
		return nil, nil, ctx.Err()
	}

	return cn, cn, nil
}

// ServerHandshake will always panic. We don't support running uTLS as server.
func (c *grpcTLS) ServerHandshake(net.Conn) (net.Conn, credentials.AuthInfo, error) {
	panic("not available!")
}

func (c *grpcTLS) Clone() credentials.TransportCredentials {
	return NewGrpcTLS(c.config, c.fingerprint)
}

func (c *grpcTLS) OverrideServerName(serverNameOverride string) error {
	c.config.ServerName = serverNameOverride
	return nil
}

// NewGrpcTLS uses c to construct a TransportCredentials based on uTLS.
func NewGrpcTLS(c *tls.Config, fingerprint string) credentials.TransportCredentials {
	tc := &grpcTLS{c.Clone(), fingerprint}
	return tc
}
