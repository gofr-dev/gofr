package errors

import (
	"fmt"
	"strings"
)

type MissingTemplate struct {
	FileLocation string
	FileName     string
}

// Error returns an error message regarding the absence of filename or existence of template
func (e MissingTemplate) Error() string {
	if strings.TrimSpace(e.FileName) == "" {
		return "Filename not provided"
	}

	return fmt.Sprintf("Template %v does not exist at location: %v", e.FileName, e.FileLocation)
}
