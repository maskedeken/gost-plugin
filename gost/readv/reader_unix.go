// +build illumos

package readv

import "golang.org/x/sys/unix"

func Read(fd uintptr, buf []byte) (n int) {
	var err error
	n, err = unix.Readv(int(fd), [][]byte{buf})
	if err != nil {
		return -1
	}

	return
}
