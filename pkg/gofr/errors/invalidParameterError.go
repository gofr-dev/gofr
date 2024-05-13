package errors

import (
	"fmt"
	"net/http"
	"strings"
)

type InvalidParamError struct {
	Param []string `json:"param,omitempty"`
}

func (e *InvalidParamError) Error() string {
	if len(e.Param) == 1 {
		return fmt.Sprintf("Parameter '%s' is invalid", e.Param[0])
	} else if len(e.Param) > 1 {
		paramList := strings.Join(e.Param, ", ")
		return fmt.Sprintf("Parameters %s are invalid", paramList)
	}
	// Handle case of empty Param slice (optional)
	return "This request has invalid parameters"
}

func (e *InvalidParamError) StatusCode() int {
	return http.StatusBadRequest
}
