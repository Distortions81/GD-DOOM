//go:build linux || darwin || freebsd || openbsd || netbsd || dragonfly

package roomhost

import (
	"golang.org/x/sys/unix"
	"os"
)

func tryLockUploadFile(file *os.File) error {
	return unix.Flock(int(file.Fd()), unix.LOCK_EX|unix.LOCK_NB)
}
