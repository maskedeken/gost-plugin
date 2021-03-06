// +build windows

package readv

import (
	"syscall"
)

func ReadRaw(fd uintptr, buf []byte) (n int) {
	size := uint32(len(buf))
	if size <= 0 {
		return
	}

	var nBytes uint32
	var flags uint32
	wsaBuffer := syscall.WSABuf{Len: size, Buf: &buf[0]}
	err := syscall.WSARecv(syscall.Handle(fd), &wsaBuffer, 1, &nBytes, &flags, nil, nil)
	if err != nil {
		return -1
	}

	n = int(nBytes)
	return
}
