//go:build !plan9 && !windows

package file

import (
	"os"
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
}
