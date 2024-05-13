package errors

import (
	"fmt"
	"net/http"
)

// MethodNotAllowedError represents an error for when a method is not allowed on a URL.
type MethodNotAllowedError struct {
	Method string `json:"method"`
	URL    string `json:"url"`
}

// Error returns the formatted error message.
func (e *MethodNotAllowedError) Error() string {
	return fmt.Sprintf("Method '%s' is not allowed on URL '%s'", e.Method, e.URL)
}

func (e *MethodNotAllowedError) StatusCode() int {
	return http.StatusMethodNotAllowed
}
