// +build linux

package hook

import (
	"context"
	"syscall"

	"github.com/maskedeken/gost-plugin/args"
	C "github.com/maskedeken/gost-plugin/constant"
	"github.com/maskedeken/gost-plugin/registry"
)

const (
	// For incoming connections.
	TCP_FASTOPEN = 23 // nolint: golint,stylecheck
	// For out-going connections.
	TCP_FASTOPEN_CONNECT = 30 // nolint: golint,stylecheck
)

func SetOutboundTFO(ctx context.Context, network string, address string, s uintptr) error {
	options := ctx.Value(C.OPTIONS).(*args.Options)
	if options.FastOpen {
		return syscall.SetsockoptInt(int(s), syscall.SOL_TCP, TCP_FASTOPEN_CONNECT, 1)
	}

	return nil
}

func SetInboundTFO(ctx context.Context, network string, address string, s uintptr) error {
	options := ctx.Value(C.OPTIONS).(*args.Options)
	if options.FastOpen {
		return syscall.SetsockoptInt(int(s), syscall.SOL_TCP, TCP_FASTOPEN, 1)
	}

	return nil
}

func init() {
	registry.RegisterDialController(SetOutboundTFO)
	registry.RegisterListenController(SetInboundTFO)
}
