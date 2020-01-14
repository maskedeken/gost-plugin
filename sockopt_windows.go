package main

import "syscall"

const (
	TCP_FASTOPEN = 15
)

func applyOutboundSocketOptions(fd uintptr, wsOpts *WSOptions) error {
	if wsOpts.Fastopen {
		if err := syscall.SetsockoptInt(syscall.Handle(fd), syscall.IPPROTO_IP, TCP_FASTOPEN, 1); err != nil {
			return newError("failed to set TCP_FASTOPEN_CONNECT=1")
		}
	}

	return nil
}

func applyInboundSocketOptions(fd uintptr, wsOpts *WSOptions) error {
	if wsOpts.Fastopen {
		if err := syscall.SetsockoptInt(syscall.Handle(fd), syscall.IPPROTO_IP, TCP_FASTOPEN, 1); err != nil {
			return newError("failed to set TCP_FASTOPEN_CONNECT=1")
		}
	}

	return nil
}
