package main

import (
	"fmt"
	"log"
)

// Logf uses the standard log library log.Output
func Logf(format string, v ...interface{}) {
	log.Output(3, fmt.Sprintf(format, v...))
}