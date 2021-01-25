package gost

import "context"

type Controller func(ctx context.Context, network, address string, fd uintptr) error
