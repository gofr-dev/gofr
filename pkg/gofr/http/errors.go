// Package http provides a set of utilities for handling HTTP requests and responses within the GoFr framework.
package http

import (
	"fmt"
	"net/http"
	"strings"
)

const alreadyExistsMessage = "entity already exists"

// ErrorEntityNotFound represents an error for when an entity is not found in the system.
type ErrorEntityNotFound struct {
	Name  string
	Value string
}

func (e ErrorEntityNotFound) Error() string {
	// For ex: "No entity found with id: 2"
	return fmt.Sprintf("No entity found with %s: %s", e.Name, e.Value)
}

func (ErrorEntityNotFound) StatusCode() int {
	return http.StatusNotFound
}

// ErrorEntityAlreadyExist represents an error for when entity is already present in the storage and we are trying to make duplicate entry.
type ErrorEntityAlreadyExist struct {
}

func (ErrorEntityAlreadyExist) Error() string {
	return alreadyExistsMessage
}

func (ErrorEntityAlreadyExist) StatusCode() int {
	return http.StatusConflict
}

// ErrorInvalidParam represents an error for invalid parameter values.
type ErrorInvalidParam struct {
	Params []string `json:"param,omitempty"` // Params contains the list of invalid parameter names.
}

func (e ErrorInvalidParam) Error() string {
	return fmt.Sprintf("'%d' invalid parameter(s): %s", len(e.Params), strings.Join(e.Params, ", "))
}

func (ErrorInvalidParam) StatusCode() int {
	return http.StatusBadRequest
}

// ErrorMissingParam represents an error for missing parameters in a request.
type ErrorMissingParam struct {
	Params []string `json:"param,omitempty"`
}

func (e ErrorMissingParam) Error() string {
	return fmt.Sprintf("'%d' missing parameter(s): %s", len(e.Params), strings.Join(e.Params, ", "))
}

func (ErrorMissingParam) StatusCode() int {
	return http.StatusBadRequest
}

// ErrorInvalidRoute represents an error for invalid route in a request.
type ErrorInvalidRoute struct{}

func (ErrorInvalidRoute) Error() string {
	return "route not registered"
}

func (ErrorInvalidRoute) StatusCode() int {
	return http.StatusNotFound
}

// ErrorRequestTimeout represents an error for request which timed out.
type ErrorRequestTimeout struct{}

func (ErrorRequestTimeout) Error() string {
	return "request timed out"
}

func (ErrorRequestTimeout) StatusCode() int {
	return http.StatusRequestTimeout
}

// ErrorPanicRecovery represents an error for request which panicked.
type ErrorPanicRecovery struct{}

func (ErrorPanicRecovery) Error() string {
	return http.StatusText(http.StatusInternalServerError)
}

func (ErrorPanicRecovery) StatusCode() int {
	return http.StatusInternalServerError
}
