package registry

import (
	"context"
	"errors"

	"github.com/maskedeken/gost-plugin/gost"
)

var (
	listenerRegistry    = make(map[string]ListenerCreator)
	transporterRegistry = make(map[string]TransporterCreator)

	errListenerNotFound    = errors.New("Listener not found")
	errTransporterNotFound = errors.New("Transporter not found")
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
