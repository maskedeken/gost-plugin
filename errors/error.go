package errors

import (
	"strings"
)

func IsEOF(err error) bool {
	errStr := err.Error()
	if strings.Contains(errStr, "EOF") {
		return true
	}

	return false
}

func IsClosed(err error) bool {
	errStr := err.Error()
	if strings.Contains(errStr, "closed") {
		return true
	}

	return false
}
