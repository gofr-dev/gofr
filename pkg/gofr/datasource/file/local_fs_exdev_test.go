//go:build !plan9

package file

import (
	"os"
	"runtime"
	"syscall"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestIsCrossDeviceError(t *testing.T) {
	crossDevice := &os.LinkError{Op: "rename", Old: "/tmp/a", New: "/data/a", Err: syscall.EXDEV}

	assert.True(t, isCrossDeviceError(crossDevice))
	assert.False(t, isCrossDeviceError(&os.LinkError{Op: "rename", Old: "a", New: "b", Err: syscall.ENOENT}))
	assert.False(t, isCrossDeviceError(os.ErrPermission))
	assert.False(t, isCrossDeviceError(nil))

	// Errno 17 is a cross-device move only on Windows.
	notSameDevice := &os.LinkError{Op: "rename", Old: `C:\a`, New: `D:\a`, Err: errorNotSameDevice}
	assert.Equal(t, runtime.GOOS == "windows", isCrossDeviceError(notSameDevice))
}
