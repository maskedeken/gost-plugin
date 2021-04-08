// +build !windows
// +build !wasm
// +build !illumos

package readv

import (
	"syscall"
	"unsafe"
)

func ReadRaw(fd uintptr, buf []byte) int {
	size := len(buf)
	if size <= 0 {
		return 0
	}

	vec := syscall.Iovec{
		Base: &buf[0],
	}
	vec.SetLen(size)
	n, _, e := syscall.Syscall(syscall.SYS_READV, fd, uintptr(unsafe.Pointer(&vec)), uintptr(1))
	if e != 0 {
		return -1
	}

	return int(n)
}
