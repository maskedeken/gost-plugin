package client

import (
	"context"
	"crypto/tls"
	"errors"
	"io"
	"net"
	"net/http"
	"net/url"
	"strconv"

	"github.com/maskedeken/gost-plugin/args"
	C "github.com/maskedeken/gost-plugin/constant"
	"github.com/maskedeken/gost-plugin/gost"
	H "github.com/maskedeken/gost-plugin/gost/protocol/http"
	"github.com/maskedeken/gost-plugin/gost/proxy"
	"github.com/maskedeken/gost-plugin/registry"
	"golang.org/x/net/http2"
)

// H2Transporter is Transporter which handles http/2
type H2Transporter struct {
	*proxy.TCPTransporter
	ctx    context.Context
	client *http.Client
}

// DialConn implements gost.Transporter.DialConn()
func (t *H2Transporter) DialConn() (net.Conn, error) {
	options := t.ctx.Value(C.OPTIONS).(*args.Options)
	reader, writer := io.Pipe()
	request := &http.Request{
		Method: "PUT",
		Body:   reader,
		URL: &url.URL{
			Scheme: "https",
			Host:   options.GetRemoteAddr(),
			Path:   options.Path,
		},
		Proto:      "HTTP/2",
		ProtoMajor: 2,
		ProtoMinor: 0,
		Header:     make(http.Header),
	}
	if options.Hostname != "" {
		request.Host = options.Hostname
	}
	// Disable any compression method from server.
	request.Header.Set("Accept-Encoding", "identity")

	response, err := t.client.Do(request)
	if err != nil {
		return nil, err
	}
	if response.StatusCode != 200 {
		return nil, errors.New("unexpected status " + strconv.Itoa(response.StatusCode))
	}

	return H.NewHttpConnection(response.Body, writer, chainedClosable{reader, writer, response.Body}, nil, nil), nil

}

// NewH2Transporter is the constructor for H2Transporter
func NewH2Transporter(ctx context.Context) (gost.Transporter, error) {
	inner, err := proxy.NewTCPTransporter(ctx)
	if err != nil {
		return nil, err
	}

	tlsConfig := buildClientTLSConfig(ctx)
	tlsConfig.NextProtos = []string{"h2"}
	transport := &http2.Transport{
		DialTLS: func(network string, addr string, tlsConfig *tls.Config) (net.Conn, error) {
			pconn, err := inner.DialConn()
			if err != nil {
				return nil, err
			}

			cn := tls.Client(pconn, tlsConfig)
			if err := cn.Handshake(); err != nil {
				return nil, err
			}
			state := cn.ConnectionState()
			if p := state.NegotiatedProtocol; p != http2.NextProtoTLS {
				return nil, errors.New("http2: unexpected ALPN protocol " + p + "; want " + http2.NextProtoTLS)
			}
			if !state.NegotiatedProtocolIsMutual {
				return nil, errors.New("http2: could not negotiate protocol mutually")
			}
			return cn, nil
		},
		TLSClientConfig: tlsConfig,
	}

	client := &http.Client{
		Transport: transport,
	}

	return &H2Transporter{
		TCPTransporter: inner.(*proxy.TCPTransporter),
		ctx:            ctx,
		client:         client,
	}, nil
}

type chainedClosable []io.Closer

// Close implements io.Closer.Close().
func (cc chainedClosable) Close() error {
	for _, c := range cc {
		_ = c.Close()
	}
	return nil
}

func init() {
	registry.RegisterTransporter("h2", NewH2Transporter)
}
