package errors

import (
	"fmt"
	"net/http"
	"strings"
)

// MissingParamError represents an error for missing parameters in a request.
type MissingParamError struct {
	Param []string `json:"param,omitempty"`
}

// Error returns the formatted error message.
func (e *MissingParamError) Error() string {
	if len(e.Param) == 0 {
		return "This request is missing parameters"
	}

	paramCount := len(e.Param)
	paramList := strings.Join(e.Param, ", ")

	return fmt.Sprintf("%d parameter(s) %s are missing for this request", paramCount, paramList)
}

func (e *MissingParamError) StatusCode() int {
	return http.StatusBadRequest
}
