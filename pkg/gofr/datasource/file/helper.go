package file

import (
	"errors"
	"fmt"
	"path"
	"strings"

	"google.golang.org/api/googleapi"
)

// IsAlreadyExistsError checks if an error indicates an object already exists.
// Handles provider-specific error codes (e.g., GCS 409/412, S3 ResourceExists).
func IsAlreadyExistsError(err error) bool {
	if err == nil {
		return false
	}

	// GCS-specific error codes
	var gErr *googleapi.Error
	if errors.As(err, &gErr) {
		return gErr.Code == 409 || gErr.Code == 412
	}

	// Fallback: check error message
	errMsg := strings.ToLower(err.Error())

	return strings.Contains(errMsg, "already exists") ||
		strings.Contains(errMsg, "resource exists") ||
		strings.Contains(errMsg, "409") ||
		strings.Contains(errMsg, "412")
}

// GenerateCopyName creates a new file name with " copy N" suffix.
// Example: "file.txt" -> "file copy 1.txt".
func GenerateCopyName(original string, count int) string {
	ext := path.Ext(original)
	base := strings.TrimSuffix(original, ext)

	return fmt.Sprintf("%s copy %d%s", base, count, ext)
}
