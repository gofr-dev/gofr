package http

import (
	"fmt"
	"net/http"
	"strings"
)

// ErrorEntityNotFound represents an error for when an entity is not found in the system.
type ErrorEntityNotFound struct {
	FieldName  string
	FieldValue string
}

// ErrorInvalidParam represents an error for invalid parameter values.
type ErrorInvalidParam struct {
	Param []string `json:"param,omitempty"` // Param contains the list of invalid parameter names.
}

// ErrorMethodNotAllowed represents an error for when a method is not allowed on a URL.
type ErrorMethodNotAllowed struct {
	Method string `json:"method"`
	URL    string `json:"url"`
}

// ErrorMissingParam represents an error for missing parameters in a request.
type ErrorMissingParam struct {
	Param []string `json:"param,omitempty"`
}

func (e *ErrorInvalidParam) Error() string {
	if len(e.Param) == 1 {
		return fmt.Sprintf("Parameter '%s' is invalid", e.Param[0])
	} else if len(e.Param) > 1 {
		paramList := strings.Join(e.Param, ", ")
		return fmt.Sprintf("Parameters %s are invalid", paramList)
	}
	// Handle case of empty Param slice (optional)
	return "This request has invalid parameters"
}

func (e *ErrorEntityNotFound) Error() string {
	// for ex: "No entity found with id : 2"
	return fmt.Sprintf("No entity found with %s : %s", e.FieldName, e.FieldValue)
}

func (e *ErrorMethodNotAllowed) Error() string {
	return fmt.Sprintf("Method '%s' is not allowed on URL '%s'", e.Method, e.URL)
}

func (e *ErrorMissingParam) Error() string {
	if len(e.Param) == 0 {
		return "This request is missing parameters"
	}

	paramCount := len(e.Param)
	paramList := strings.Join(e.Param, ", ")

	return fmt.Sprintf("%d parameter(s) %s are missing for this request", paramCount, paramList)
}

func (e *ErrorInvalidParam) StatusCode() int {
	return http.StatusBadRequest
}

func (e *ErrorEntityNotFound) StatusCode() int {
	return http.StatusNotFound
}

func (e *ErrorMethodNotAllowed) StatusCode() int {
	return http.StatusMethodNotAllowed
}

func (e *ErrorMissingParam) StatusCode() int {
	return http.StatusBadRequest
}
