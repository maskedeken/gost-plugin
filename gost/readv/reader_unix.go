// +build illumos

package readv

import "golang.org/x/sys/unix"

func Read(fd uintptr, buf []byte) (int, error) {
	return unix.Readv(int(fd), [][]byte{buf})
}
