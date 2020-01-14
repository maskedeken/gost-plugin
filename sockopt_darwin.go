package main

import "syscall"

const (
	// TCP_FASTOPEN is the socket option on darwin for TCP fast open.
	TCP_FASTOPEN = 0x105
	// TCP_FASTOPEN_SERVER is the value to enable TCP fast open on darwin for server connections.
	TCP_FASTOPEN_SERVER = 0x01
	// TCP_FASTOPEN_CLIENT is the value to enable TCP fast open on darwin for client connections.
	TCP_FASTOPEN_CLIENT = 0x02
)

func applyOutboundSocketOptions(fd uintptr, wsOpts *WSOptions) error {
	if wsOpts.Fastopen {
		if err := syscall.SetsockoptInt(int(fd), syscall.IPPROTO_TCP, TCP_FASTOPEN, TCP_FASTOPEN_CLIENT); err != nil {
			return err
		}
	}

	return nil
}

func applyInboundSocketOptions(fd uintptr, wsOpts *WSOptions) error {
	if wsOpts.Fastopen {
		if err := syscall.SetsockoptInt(int(fd), syscall.IPPROTO_TCP, TCP_FASTOPEN, TCP_FASTOPEN_SERVER); err != nil {
			return err
		}
	}

	return nil
}