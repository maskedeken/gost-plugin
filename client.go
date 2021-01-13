package main

import (
	"context"
	"crypto/tls"
	"fmt"
	"io"
	"log"
	"net"
	"net/url"
)

var (
	globalSessionCache = tls.NewLRUClientSessionCache(128)
)

type Client struct {
	ctx         context.Context
	listener    net.Listener
	transporter requestTransporter
}

func NewClient(ctx context.Context, opts Options) (c *Client, err error) {
	addr := fmt.Sprintf("%s:%d", fixAddr(opts.localAddr), opts.localPort)
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		return
	}

	c = &Client{
		ctx:      ctx,
		listener: ln,
	}

	var tlsConfig *tls.Config
	protocol := "ws"
	if opts.tlsEnabled {
		protocol = "wss"
		tlsConfig = &tls.Config{
			ClientSessionCache:     globalSessionCache,
			NextProtos:             []string{"http/1.1"},
			InsecureSkipVerify:     opts.insecure,
			SessionTicketsDisabled: true,
		}
		if opts.hostname != "" {
			tlsConfig.ServerName = opts.hostname
		}
	}
	url := url.URL{Scheme: protocol, Host: fmt.Sprintf("%s:%d", fixAddr(opts.remoteAddr), opts.remotePort), Path: opts.path}

	if opts.mode == "mws" {
		pool := newMuxPool(ctx, opts.mux)
		c.transporter = newMWSTransporter(url, tlsConfig, !opts.nocomp, pool)
	} else {
		c.transporter = newWSTransporter(url, tlsConfig, !opts.nocomp)
	}

	go func() {
		for {
			conn, err := ln.Accept()
			if err != nil {
				log.Printf("failed to accept connection: %s", err.Error())
				select {
				case <-c.ctx.Done():
					return
				default:
					continue
				}
			}

			go c.handleConn(conn)
		}
	}()

	return
}

func (c *Client) Close() error {
	return c.listener.Close()
}

func (c *Client) handleConn(inbound net.Conn) {
	defer inbound.Close()

	outbound, err := c.transporter.DialConn()
	if err != nil {
		log.Printf("failed to dial connection: %s", err.Error())
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
	if err != nil && cause(err) != io.EOF {
		log.Printf("failed to transfer request: %s", err.Error())
	}
}

func (c *Client) Shutdown() error {
	return c.Close()
}
