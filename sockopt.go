package main

import "syscall"

func getDialerControlFunc(wsOpts *WSOptions) func(network, address string, c syscall.RawConn) error {
	return func(network, address string, c syscall.RawConn) error {
		return c.Control(func(fd uintptr) {
			applyOutboundSocketOptions(fd, wsOpts)
		})
	}
}

func getListenerControlFunc(wsOpts *WSOptions) func(network, address string, c syscall.RawConn) error {
	return func(network, address string, c syscall.RawConn) error {
		return c.Control(func(fd uintptr) {
			applyInboundSocketOptions(fd, wsOpts)
		})
	}
}
