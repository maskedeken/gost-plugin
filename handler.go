package main

import (
	"context"
	"io"
	"net"

	"github.com/maskedeken/gost-plugin/registry"

	"github.com/maskedeken/gost-plugin/args"
	C "github.com/maskedeken/gost-plugin/constant"
	"github.com/maskedeken/gost-plugin/errors"
	"github.com/maskedeken/gost-plugin/gost"
	"github.com/maskedeken/gost-plugin/gost/protocol"
	"github.com/maskedeken/gost-plugin/log"
)

type Handler struct {
	listener    gost.Listener
	transporter gost.Transporter
}

func (h *Handler) Close() (err error) {
	return h.listener.Close()
}

func (h *Handler) Serve(ctx context.Context) {
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
				copyConn := func(a, b net.Conn) {
					_, err := io.Copy(a, b)
					errChan <- err
					return
				}
				go copyConn(inbound, outbound)
				go copyConn(outbound, inbound)

				err = <-errChan
				if err != nil && !errors.IsEOF(err) && !errors.IsClosed(err) {
					log.Errorf("connection ends with error: %s", err)
				}
			}(conn)
		}
	}()

	go h.listener.Serve(ctx)
}

func NewHandler(ctx context.Context) (*Handler, error) {
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

		transporter, err = protocol.NewTCPTransporter(ctx)
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

		listener, err = protocol.NewTCPListener(ctx)
		if err != nil {
			return nil, err
		}

	}

	return &Handler{
		listener:    listener,
		transporter: transporter,
	}, nil
}
