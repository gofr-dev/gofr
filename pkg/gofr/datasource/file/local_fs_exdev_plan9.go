//go:build plan9

package file

// isCrossDeviceError reports false: plan9 has no EXDEV error to detect.
func isCrossDeviceError(error) bool {
	return false
}
