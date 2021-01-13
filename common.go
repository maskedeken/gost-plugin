package main

import "strings"

type Options struct {
	localAddr  string
	localPort  uint
	remoteAddr string
	remotePort uint
	hostname   string
	mode       string
	path       string
	tlsEnabled bool
	server     bool
	nocomp     bool
	insecure   bool
	cert       string
	key        string
	mux        uint
}

type Worker interface {
	Shutdown() error
}

func fixAddr(addr string) string {
	size := len(addr)
	if size == 0 {
		return addr
	}

	if strings.Contains(addr, ":") && addr[0] != '[' && addr[size-1] != ']' {
		return "[" + addr + "]"
	}

	return addr
}
