package errors

import (
	"fmt"
	"strings"
)

// MultipleErrors is used when more than one error needs to be returned for a single request
type MultipleErrors struct {
	StatusCode int     `json:"-" xml:"-"`
	Errors     []error `json:"errors" xml:"errors"`
}

// Error returns a formatted error message by concatenating multiple error messages
func (m MultipleErrors) Error() string {
	var result string

	for _, v := range m.Errors {
		result += fmt.Sprintf("%s\n", v)
	}

	return strings.TrimSuffix(result, "\n")
}
