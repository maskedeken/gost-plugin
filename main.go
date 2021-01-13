package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"runtime"
	"syscall"
)

var (
	VERSION = "unknown"
)

var (
	options = Options{}
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
	flags.StringVar(&options.localAddr, "localAddr", "127.0.0.1", "local address to listen on.")
	flags.UintVar(&options.localPort, "localPort", 1984, "local port to listen on.")
	flags.StringVar(&options.remoteAddr, "remoteAddr", "127.0.0.1", "remote address to forward.")
	flags.UintVar(&options.remotePort, "remotePort", 1080, "remote port to forward.")
	flags.StringVar(&options.path, "path", "/", "URL path for websocket.")
	flags.StringVar(&options.hostname, "host", "", "Hostname for server.")
	flags.StringVar(&options.cert, "cert", "", "Path to TLS certificate file.")
	flags.StringVar(&options.key, "key", "", "(server) Path to TLS key file.")
	flags.StringVar(&options.mode, "mode", "ws", "Transport mode = ws(Websocket) mws(Multiplex Websocket).")
	flags.BoolVar(&options.server, "server", false, "Run in server mode.")
	flags.BoolVar(&options.tlsEnabled, "tls", false, "Enable TLS.")
	flags.BoolVar(&options.nocomp, "nocomp", false, "Disable compression.")
	flags.BoolVar(&options.insecure, "insecure", false, "Allow insecure TLS connections.")
	flags.UintVar(&options.mux, "mux", 1, "MUX sessions for Multiplex Websocket.")

	flags.Parse(os.Args[1:])
	if *version {
		printVersion()
		return
	}

	err = parseOpts(&options)
	if err != nil {
		log.Fatal(err.Error())
		os.Exit(23)
	}

	var worker Worker
	ctx := context.Background()
	ctx, cancel := context.WithCancel(ctx)
	if options.server { // server mode
		worker, err = NewServer(ctx, options)
		if err != nil {
			log.Fatalf("failed to start server: %s", err.Error())
			os.Exit(1)
		}

	} else { // client mode
		worker, err = NewClient(ctx, options)
		if err != nil {
			log.Fatalf("failed to start client: %s", err.Error())
			os.Exit(1)
		}
	}

	defer func() {
		cancel()
		err := worker.Shutdown()
		if err != nil {
			log.Println(err.Error())
		}
	}()

	{
		osSignals := make(chan os.Signal, 1)
		signal.Notify(osSignals, os.Interrupt, os.Kill, syscall.SIGTERM)
		<-osSignals
	}
}
