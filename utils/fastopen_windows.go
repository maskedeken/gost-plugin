// +build windows

package utils

import (
	"context"
	"syscall"

	"github.com/maskedeken/gost-plugin/args"
	C "github.com/maskedeken/gost-plugin/constant"
	"github.com/maskedeken/gost-plugin/registry"
)

const (
	TCP_FASTOPEN = 15 // nolint: golint,stylecheck
)

func SetTFO(ctx context.Context, network string, address string, s uintptr) error {
	options := ctx.Value(C.OPTIONS).(*args.Options)
	if options.FastOpen {
		fd := syscall.Handle(s)
		return syscall.SetsockoptInt(fd, syscall.IPPROTO_TCP, TCP_FASTOPEN, 1)
	}

	return nil
}

func init() {
	registry.RegisterDialController(SetTFO)
	registry.RegisterListenController(SetTFO)
}
