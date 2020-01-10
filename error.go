package main

import (
	"fmt"
	"strings"
)

type Error struct {
	message []interface{}
}

func (err *Error) Error() string {
	builder := strings.Builder{}
	for _, value := range err.message {
		builder.WriteString(toString(value))
	}
	return builder.String()
}

// ToString serialize an arbitrary value into string.
func toString(v interface{}) string {
	if v == nil {
		return " "
	}

	switch value := v.(type) {
	case string:
		return value
	case *string:
		return *value
	case fmt.Stringer:
		return value.String()
	case error:
		return value.Error()
	default:
		return fmt.Sprintf("%+v", value)
	}
}

func newError(msg ...interface{}) *Error {
	return &Error{message: msg}
}
