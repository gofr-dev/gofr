package errors

import (
	"fmt"
	"net/http"
)

type MethodNotAllowedError struct {
	Method string `json:"method"`
	URL    string `json:"url"`
}

func (e *MethodNotAllowedError) Error() string {
	return fmt.Sprintf("Method '%s' is not allowed on URL '%s'", e.Method, e.URL)
}

func (e *MethodNotAllowedError) StatusCode() int {
	return http.StatusMethodNotAllowed
}
