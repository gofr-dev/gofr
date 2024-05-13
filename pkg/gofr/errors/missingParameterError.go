package errors

import (
	"fmt"
	"net/http"
	"strings"
)

type MissingParamError struct {
	Param []string `json:"param,omitempty"`
}

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
