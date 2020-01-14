package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"runtime"
	"syscall"
)

var (
	VERSION = "20200110"
)

var (
	options = &Options{}
)

func init() {
}

func printVersion() {
	fmt.Println("gost-plugin", VERSION)
	fmt.Println("Go version", runtime.Version())
	fmt.Println("Yet another SIP003 plugin for shadowsocks")
}

func main() {
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
	flags.BoolVar(&options.fastopen, "fast-open", false, "Enable TCP Fast Open.")

	flags.Parse(os.Args[1:])
	if *version {
		printVersion()
		return
	}

	err := parseOpts(options)
	if err != nil {
		log.Fatal(err.Error())
		os.Exit(23)
	}

	router, err := NewRouter(options)
	if err != nil {
		log.Fatal("failed to start server:", err.Error())
		os.Exit(1)
	}

	go router.Serve()

	defer func() {
		err := router.Close()
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
