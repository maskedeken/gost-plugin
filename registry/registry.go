package registry

import (
	"context"
	"errors"
	"syscall"

	"github.com/maskedeken/gost-plugin/gost"
	"github.com/maskedeken/gost-plugin/log"
)

var (
	listenerRegistry    = make(map[string]ListenerCreator)
	transporterRegistry = make(map[string]TransporterCreator)

	dialControllers   []gost.Controller
	listenControllers []gost.Controller

	errListenerNotFound    = errors.New("Listener not found")
	errTransporterNotFound = errors.New("Transporter not found")

	errControllerIsNil = errors.New("Nil Controller")
)

type ListenerCreator func(context.Context) (gost.Listener, error)
type TransporterCreator func(context.Context) (gost.Transporter, error)

func RegisterListener(name string, l ListenerCreator) {
	listenerRegistry[name] = l
}

func RegisterTransporter(name string, t TransporterCreator) {
	transporterRegistry[name] = t
}

func GetListenerCreator(name string) (ListenerCreator, error) {
	l, ok := listenerRegistry[name]
	if !ok {
		return nil, errListenerNotFound
	}

	return l, nil
}

func GetTransporterCreator(name string) (TransporterCreator, error) {
	t, ok := transporterRegistry[name]
	if !ok {
		return nil, errTransporterNotFound
	}

	return t, nil
}

func RegisterDialController(controlFunc gost.Controller) error {
	if controlFunc == nil {
		return errControllerIsNil
	}

	dialControllers = append(dialControllers, controlFunc)
	return nil
}

func RegisterListenController(controlFunc gost.Controller) error {
	if controlFunc == nil {
		return errControllerIsNil
	}

	listenControllers = append(listenControllers, controlFunc)
	return nil
}

func GetDialControl(ctx context.Context) func(string, string, syscall.RawConn) error {
	return func(network, address string, c syscall.RawConn) error {
		return c.Control(func(fd uintptr) {
			for _, ctl := range dialControllers {
				if err := ctl(ctx, network, address, fd); err != nil {
					log.Errorf("failed to apply dail controller: %s", err)
				}
			}
		})
	}
}

func GetListenControl(ctx context.Context) func(string, string, syscall.RawConn) error {
	return func(network, address string, c syscall.RawConn) error {
		return c.Control(func(fd uintptr) {
			for _, ctl := range listenControllers {
				if err := ctl(ctx, network, address, fd); err != nil {
					log.Errorf("failed to apply listen controller: %s", err)
				}
			}
		})
	}
}
