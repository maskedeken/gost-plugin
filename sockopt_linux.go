package main

import "syscall"

const (
	// For incoming connections.
	TCP_FASTOPEN = 23
	// For out-going connections.
	TCP_FASTOPEN_CONNECT = 30
)

func applyOutboundSocketOptions(fd uintptr, wsOpts *WSOptions) error {
	if wsOpts.Fastopen {
		if err := syscall.SetsockoptInt(int(fd), syscall.SOL_TCP, TCP_FASTOPEN_CONNECT, 1); err != nil {
			return newError("failed to set TCP_FASTOPEN_CONNECT=1")
		}
	}

	return nil
}

func applyInboundSocketOptions(fd uintptr, wsOpts *WSOptions) error {
	if wsOpts.Fastopen {
		if err := syscall.SetsockoptInt(int(fd), syscall.SOL_TCP, TCP_FASTOPEN, 1); err != nil {
			return newError("failed to set TCP_FASTOPEN=1")
		}
	}

	return nil
}