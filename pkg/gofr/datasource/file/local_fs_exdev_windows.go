//go:build windows

package file

import (
	"errors"
	"syscall"
)

// errorNotSameDevice is ERROR_NOT_SAME_DEVICE, which Windows returns for a move across
// volumes. syscall.EXDEV is never returned on Windows.
const errorNotSameDevice syscall.Errno = 17

// isCrossDeviceError reports whether err is a rename failing because the source and
// destination are on different volumes.
func isCrossDeviceError(err error) bool {
	return errors.Is(err, errorNotSameDevice)
}
