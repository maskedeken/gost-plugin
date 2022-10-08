package main

import (
	"context"
	"io"
	"net"

	"github.com/maskedeken/gost-plugin/gost/proxy"
	"github.com/maskedeken/gost-plugin/registry"
	xtls "github.com/xtls/go"

	"github.com/maskedeken/gost-plugin/args"
	C "github.com/maskedeken/gost-plugin/constant"
	"github.com/maskedeken/gost-plugin/gost"
	"github.com/maskedeken/gost-plugin/log"
)

var defaultReadSize int64 = 2048

type handler struct {
	listener    gost.Listener
	transporter gost.Transporter
}

func (h *handler) Close() (err error) {
	return h.listener.Close()
}

func (h *handler) Serve(ctx context.Context) {
	go func() {
		for {
			conn, err := h.listener.AcceptConn()
			if err != nil {
				log.Errorf("failed to accept connection: %s", err)
				select {
				case <-ctx.Done():
					return
				default:
					continue
				}
			}

			go func(inbound net.Conn) {
				defer inbound.Close()

				outbound, err := h.transporter.DialConn()
				if err != nil {
					log.Errorf("failed to dial connection: %s", err)
					return
				}

				defer outbound.Close()

				errChan := make(chan error, 2)
				copy := func(a, b net.Conn) {
					_, err := copyConn(a, b)
					errChan <- err
					return
				}
				go copy(inbound, outbound)
				go copy(outbound, inbound)

				err = <-errChan
				if err != nil {
					log.Debugf("connection ends with error: %s", err)
				}
			}(conn)
		}
	}()

	go h.listener.Serve(ctx)
}

func copyConn(dst io.Writer, src io.Reader) (n int64, err error) {
	var xc *xtls.Conn
	var tc *net.TCPConn
	if conn, ok := src.(interface{ GetXTLSConn() *xtls.Conn }); ok {
		xc = conn.GetXTLSConn()
	}
	if conn, ok := dst.(*net.TCPConn); ok {
		tc = conn
	}

	if tc != nil && xc != nil {
		var nn int64
		for {
			if xc.DirectIn {
				nn, err = tc.ReadFrom(xc.NetConn()) // splice
				n += nn
				return
			}

			nn, err = io.CopyN(dst, src, defaultReadSize)
			n += nn
			if err != nil {
				return
			}
		}
	}

	n, err = io.Copy(dst, src)
	return
}

func newHandler(ctx context.Context) (*handler, error) {
	var listener gost.Listener
	var transporter gost.Transporter
	var err error

	options := ctx.Value(C.OPTIONS).(*args.Options)
	if options.Server { // server mode
		log.Warnln("running in server mode")
		var newListener registry.ListenerCreator
		newListener, err = registry.GetListenerCreator(options.Mode)
		if err != nil {
			return nil, err
		}

		listener, err = newListener(ctx)
		if err != nil {
			return nil, err
		}

		transporter, err = proxy.NewTCPTransporter(ctx)
		if err != nil {
			return nil, err
		}

	} else { // client mode
		log.Warnln("running in client mode")
		var newTransporter registry.TransporterCreator
		newTransporter, err = registry.GetTransporterCreator(options.Mode)
		if err != nil {
			return nil, err
		}

		transporter, err = newTransporter(ctx)
		if err != nil {
			return nil, err
		}

		listener, err = proxy.NewTCPListener(ctx)
		if err != nil {
			return nil, err
		}
	}

	return &handler{
		listener:    listener,
		transporter: transporter,
	}, nil
}
