package net

import (
	"context"
	"fmt"
	"io"
	"net"
	"strings"

	"github.com/maskedeken/gost-plugin/args"
	C "github.com/maskedeken/gost-plugin/constant"
	"github.com/maskedeken/gost-plugin/errors"
	"github.com/maskedeken/gost-plugin/log"
)

type Handler struct {
	listener    Listener
	transporter Transporter
}

func (h *Handler) Close() (err error) {
	return h.listener.Close()
}

func (h *Handler) Serve(ctx context.Context) {
	go func() {
		for {
			conn, err := h.listener.AcceptConn()
			if err != nil {
				log.Errorln("failed to accept connection: %s", err)
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
					log.Errorln("failed to dial connection: %s", err)
					return
				}

				defer outbound.Close()

				errChan := make(chan error, 2)
				copyConn := func(a, b net.Conn) {
					_, err := io.Copy(a, b)
					errChan <- err
					return
				}
				go copyConn(inbound, outbound)
				go copyConn(outbound, inbound)

				err = <-errChan
				if err != nil && !errors.IsEOF(err) && !errors.IsClosed(err) {
					log.Errorln("connection ends with error: %s", err)
				}
			}(conn)
		}
	}()

	go h.listener.Serve(ctx)
}

func NewHandler(ctx context.Context) (*Handler, error) {
	var listener Listener
	var transporter Transporter
	var err error

	options := ctx.Value(C.OPTIONS).(*args.Options)
	if options.Server { // server mode
		log.Warnln("running in server mode")
		transporter, err = NewTCPTransporter(ctx)
		if err != nil {
			return nil, err
		}

		switch strings.ToLower(options.Mode) {
		case "tls":
			listener, err = NewTLSListener(ctx)
		case "mtls":
			listener, err = NewMTLSListener(ctx)
		case "ws":
			listener, err = NewWSListener(ctx)
		case "wss":
			listener, err = NewWSSListener(ctx)
		case "mws":
			listener, err = NewMWSListener(ctx)
		case "mwss":
			listener, err = NewMWSSListener(ctx)
		default:
			return nil, fmt.Errorf("invalid mode")
		}

	} else { // client mode
		log.Warnln("running in client mode")
		listener, err = NewTCPListener(ctx)
		if err != nil {
			return nil, err
		}

		switch strings.ToLower(options.Mode) {
		case "tls":
			transporter, err = NewTLSTransporter(ctx)
		case "mtls":
			transporter, err = NewMTLSTransporter(ctx)
		case "ws":
			transporter, err = NewWSTransporter(ctx)
		case "wss":
			transporter, err = NewWSSTransporter(ctx)
		case "mws":
			transporter, err = NewMWSTransporter(ctx)
		case "mwss":
			transporter, err = NewMWSSTransporter(ctx)
		default:
			return nil, fmt.Errorf("invalid mode")
		}

	}

	return &Handler{
		listener:    listener,
		transporter: transporter,
	}, nil
}
