// +build !windows
// +build !wasm
// +build !illumos

package readv

import (
	"fmt"
	"syscall"
	"unsafe"
)

func Read(fd uintptr, buf []byte) (int, error) {
	size := len(buf)
	if size <= 0 {
		return 0, nil
	}

	vec := syscall.Iovec{
		Base: &buf[0],
	}
	vec.SetLen(size)
	n, _, e := syscall.Syscall(syscall.SYS_READV, fd, uintptr(unsafe.Pointer(&vec)), uintptr(1))
	if e != 0 {
		return 0, fmt.Errorf("failed to invoke readv method.")
	}

	return int(n), nil
}
