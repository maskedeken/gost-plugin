package main

import (
	"context"
	"crypto/tls"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"time"

	"github.com/gorilla/websocket"
	"github.com/xtaci/smux"
)

const (
	KeepAliveTime = 180 * time.Second
)

type tcpKeepAliveListener struct {
	*net.TCPListener
}

func (ln tcpKeepAliveListener) Accept() (c net.Conn, err error) {
	tc, err := ln.AcceptTCP()
	if err != nil {
		return
	}

	tc.SetKeepAlive(true)
	tc.SetKeepAlivePeriod(KeepAliveTime)
	return tc, nil
}

type requestHandler struct {
	path  string
	mux   bool
	rAddr string
}

var upgrader = &websocket.Upgrader{
	ReadBufferSize:   4 * 1024,
	WriteBufferSize:  4 * 1024,
	HandshakeTimeout: time.Second * 4,
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
}

func (h *requestHandler) ServeHTTP(writer http.ResponseWriter, request *http.Request) {
	if request.URL.Path != h.path {
		writer.WriteHeader(http.StatusNotFound)
		return
	}

	conn, err := upgrader.Upgrade(writer, request, nil)
	if err != nil {
		log.Printf("failed to convert to WebSocket connection: %s", err.Error())
		return
	}

	/*remoteAddr := conn.RemoteAddr()*/
	c := websocketServerConn(conn)
	if !h.mux {
		go h.handleConn(c)
		return
	}

	h.serveMUX(c) // handle mux session
}

func (h *requestHandler) serveMUX(conn net.Conn) {
	smuxConfig := smux.DefaultConfig()
	session, err := smux.Server(conn, smuxConfig)
	if err != nil {
		log.Printf("failed to create mux session: %s", err.Error())
		return
	}

	defer session.Close()
	for {
		stream, err := session.AcceptStream()
		if err != nil {
			log.Printf("failed to open mux stream: %s", err.Error())
			return
		}

		go h.handleConn(stream)
	}
}

func (h *requestHandler) handleConn(inbound net.Conn) {
	defer inbound.Close()
	outbound, err := net.Dial("tcp", h.rAddr)
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

type Server struct {
	listener net.Listener
	errChan  chan error
}

func NewServer(ctx context.Context, opts Options) (s *Server, err error) {
	addr := fmt.Sprintf("%s:%d", fixAddr(opts.localAddr), opts.localPort)
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		return
	}

	s = &Server{
		listener: ln,
		errChan:  make(chan error),
	}
	server := http.Server{
		Handler: &requestHandler{
			path:  opts.path,
			mux:   opts.mode == "mws",
			rAddr: fmt.Sprintf("%s:%d", fixAddr(opts.remoteAddr), opts.remotePort),
		},
		ReadHeaderTimeout: time.Second * 4,
		MaxHeaderBytes:    2048,
	}

	go func() {
		defer close(s.errChan)

		tcpListener := s.listener.(*net.TCPListener)
		var listener net.Listener = tcpKeepAliveListener{tcpListener}
		if opts.tlsEnabled {
			if opts.cert == "" || opts.key == "" {
				s.errChan <- fmt.Errorf("No cert file or key file specified")
				return
			}

			cert, err := tls.LoadX509KeyPair(opts.cert, opts.key)
			if err != nil {
				s.errChan <- err
				return
			}

			tlsConfig := &tls.Config{Certificates: []tls.Certificate{cert}}
			listener = tls.NewListener(listener, tlsConfig)
		}

		if err := server.Serve(listener); err != nil {
			s.errChan <- err
		}

	}()

	err = <-s.errChan
	return
}

func (s *Server) Shutdown() error {
	return s.listener.Close()
}
