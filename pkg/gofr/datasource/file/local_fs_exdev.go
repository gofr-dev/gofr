//go:build !plan9

package file

import (
	"errors"
	"runtime"
	"syscall"
)

// errorNotSameDevice is ERROR_NOT_SAME_DEVICE, which Windows returns instead of EXDEV
// for a move across volumes. Elsewhere 17 is another error, such as EEXIST on Linux.
const errorNotSameDevice syscall.Errno = 17

// isCrossDeviceError reports whether err is a rename failing because the source and
// destination are on different filesystems.
func isCrossDeviceError(err error) bool {
	return errors.Is(err, syscall.EXDEV) ||
		runtime.GOOS == "windows" && errors.Is(err, errorNotSameDevice)
}
