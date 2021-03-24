// +build windows

package readv

import (
	"syscall"
)

func Read(fd uintptr, buf []byte) (n int, err error) {
	size := uint32(len(buf))
	if size <= 0 {
		return
	}

	var nBytes uint32
	var flags uint32
	wsaBuffer := syscall.WSABuf{Len: size, Buf: &buf[0]}
	err = syscall.WSARecv(syscall.Handle(fd), &wsaBuffer, 1, &nBytes, &flags, nil, nil)
	if err != nil {
		return
	}

	n = int(nBytes)
	return
}
