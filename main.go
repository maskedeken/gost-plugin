package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"runtime"
	"syscall"

	"github.com/maskedeken/gost-plugin/args"
	"github.com/maskedeken/gost-plugin/log"

	_ "github.com/maskedeken/gost-plugin/gost/proxy/client"
	_ "github.com/maskedeken/gost-plugin/gost/proxy/server"
	_ "github.com/maskedeken/gost-plugin/hook"
)

var (
	VERSION = "unknown"
)

var (
	options = args.Options{}
)

func init() {
}

func printVersion() {
	fmt.Println("gost-plugin", VERSION)
	fmt.Println("Go version", runtime.Version())
	fmt.Println("Yet another SIP003 plugin for shadowsocks")
}

func main() {
	var err error

	flags := flag.NewFlagSet("gost-plugin", flag.ExitOnError)
	version := flags.Bool("version", false, "Show current version of gost-plugin")
	flags.StringVar(&options.LocalAddr, "localAddr", "127.0.0.1", "local address to listen on.")
	flags.UintVar(&options.LocalPort, "localPort", 1984, "local port to listen on.")
	flags.StringVar(&options.RemoteAddr, "remoteAddr", "127.0.0.1", "remote address to forward.")
	flags.UintVar(&options.RemotePort, "remotePort", 1080, "remote port to forward.")
	flags.StringVar(&options.Path, "path", "/", "URL path for websocket/h2.")
	flags.StringVar(&options.Hostname, "host", "", "(client) Host header for websocket/h2.")
	flags.StringVar(&options.ServerName, "serverName", "", "(client) Server name for server.")
	flags.StringVar(&options.Cert, "cert", "", "(server) Path to TLS certificate file.")
	flags.StringVar(&options.Key, "key", "", "(server) Path to TLS key file.")
	flags.StringVar(&options.Mode, "mode", "ws", "Transport mode = tls, mtls, ws, wss, mws, mwss, h2, quic, grpc.")
	flags.BoolVar(&options.Server, "server", false, "Run in server mode.")
	flags.BoolVar(&options.Nocomp, "nocomp", false, "(client) Disable websocket compression.")
	flags.BoolVar(&options.Insecure, "insecure", false, "(client) Allow insecure TLS connections.")
	flags.UintVar(&options.Mux, "mux", 1, "(client) MUX sessions per Websocket connection.")
	flags.UintVar(&options.LogLevel, "logLevel", 3, "Log level (0:Panic, 1:Fatal, 2:Error, 3:Warn, 4:Info, 5:Debug, 6:Trace).")
	flags.BoolVar(&options.Vpn, "V", false, "Run in VPN mode.")
	flags.BoolVar(&options.FastOpen, "fast-open", false, "Enable TCP fast open.")
	flags.UintVar(&options.Ed, "ed", 0, "(client) Length of early data for WebSocket. WebSocket 0-RTT is enabled if ed > 0.")
	flags.StringVar(&options.ServiceName, "serviceName", "GunService", "Service name for gRPC.")
	flags.StringVar(&options.Fingerprint, "fingerprint", "", "(client) Fingerprint for TLS. Should be one of chrome, firefox, safari, edge, ios, 360browser, qqbrowser.")

	flags.Parse(os.Args[1:])
	if *version {
		printVersion()
		return
	}

	if options.LogLevel > 6 {
		options.LogLevel = 6
	}
	log.SetLevel(options.LogLevel)

	ctx := context.Background()
	ctx, cancel := context.WithCancel(ctx)
	ctx, err = args.ApplyOptions(ctx, &options)
	if err != nil {
		log.Fatalf("failed to init context: %s", err)
		os.Exit(23)
	}

	handler, err := newHandler(ctx)
	if err != nil {
		log.Fatalf("failed to create handler: %s", err)
		os.Exit(23)
	}

	handler.Serve(ctx)

	defer func() {
		cancel()
		err := handler.Close()
		if err != nil {
			log.Warnf("failed to close handler: %s", err)
		}
	}()

	{
		osSignals := make(chan os.Signal, 1)
		signal.Notify(osSignals, os.Interrupt, os.Kill, syscall.SIGTERM)
		<-osSignals
	}
}
