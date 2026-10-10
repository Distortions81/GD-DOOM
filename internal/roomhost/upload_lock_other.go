//go:build !linux && !darwin && !freebsd && !openbsd && !netbsd && !dragonfly && !windows

package roomhost

import (
	"errors"
	"os"
)

func tryLockUploadFile(*os.File) error {
	return errors.New("upload storage locking is unsupported on this platform")
}
