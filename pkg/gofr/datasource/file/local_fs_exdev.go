//go:build !plan9 && !windows

package file

import (
	"errors"
	"syscall"
)

// isCrossDeviceError reports whether err is a rename failing because the source and
// destination are on different filesystems.
func isCrossDeviceError(err error) bool {
	return errors.Is(err, syscall.EXDEV)
}
