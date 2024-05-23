// Package http provides a set of utilities for handling HTTP requests and responses within the GoFr framework.
package http

import (
	"fmt"
	"net/http"
	"strings"
)

// ErrorEntityNotFound represents an error for when an entity is not found in the system.
type ErrorEntityNotFound struct {
	Name  string
	Value string
}

func (e ErrorEntityNotFound) Error() string {
	// For ex: "No entity found with id: 2"
	return fmt.Sprintf("No entity found with %s: %s", e.Name, e.Value)
}

func (e ErrorEntityNotFound) StatusCode() int {
	return http.StatusNotFound
}

// ErrorInvalidParam represents an error for invalid parameter values.
type ErrorInvalidParam struct {
	Params []string `json:"param,omitempty"` // Params contains the list of invalid parameter names.
}

func (e ErrorInvalidParam) Error() string {
	return fmt.Sprintf("'%d' invalid parameter(s): %s", len(e.Params), strings.Join(e.Params, ", "))
}

func (e ErrorInvalidParam) StatusCode() int {
	return http.StatusBadRequest
}

// ErrorMissingParam represents an error for missing parameters in a request.
type ErrorMissingParam struct {
	Params []string `json:"param,omitempty"`
}

func (e ErrorMissingParam) Error() string {
	return fmt.Sprintf("'%d' missing parameter(s): %s", len(e.Params), strings.Join(e.Params, ", "))
}

func (e ErrorMissingParam) StatusCode() int {
	return http.StatusBadRequest
}

// ErrorInvalidRoute represents an error for invalid route in a request.
type ErrorInvalidRoute struct{}

func (e ErrorInvalidRoute) Error() string {
	return "route not registered"
}

func (e ErrorInvalidRoute) StatusCode() int {
	return http.StatusNotFound
}
