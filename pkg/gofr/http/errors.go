// Package http provides a set of utilities for handling HTTP requests and responses within the GoFr framework.
package http

import (
	"fmt"
	"net/http"
	"strings"

	"gofr.dev/pkg/gofr/logging"
)

const (
	alreadyExistsMessage      = "entity already exists"
	StatusClientClosedRequest = 499
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

func (ErrorEntityNotFound) StatusCode() int {
	return http.StatusNotFound
}

func (ErrorEntityNotFound) LogLevel() logging.Level {
	return logging.INFO
}

// ErrorEntityAlreadyExist represents an error for when entity is already present in the storage and we are trying to make duplicate entry.
type ErrorEntityAlreadyExist struct{}

func (ErrorEntityAlreadyExist) Error() string {
	return alreadyExistsMessage
}

func (ErrorEntityAlreadyExist) StatusCode() int {
	return http.StatusConflict
}

func (ErrorEntityAlreadyExist) LogLevel() logging.Level {
	return logging.WARN
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

func (ErrorInvalidParam) LogLevel() logging.Level {
	return logging.INFO
}

// ErrorMissingParam represents an error for missing parameters in a request.
type ErrorMissingParam struct {
	Params []string `json:"param,omitempty"`
}

func (e ErrorMissingParam) Error() string {
	return fmt.Sprintf("'%d' missing parameter(s): %s", len(e.Params), strings.Join(e.Params, ", "))
}

func (ErrorMissingParam) LogLevel() logging.Level {
	return logging.INFO
}

func (ErrorMissingParam) StatusCode() int {
	return http.StatusBadRequest
}

// ErrorInvalidRoute represents an error for invalid route in a request.
type ErrorInvalidRoute struct{}

func (ErrorInvalidRoute) Error() string {
	return "route not registered"
}

func (ErrorInvalidRoute) LogLevel() logging.Level {
	return logging.INFO
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
	return http.StatusGatewayTimeout // 504 is more appropriate for gateway timeouts
}

func (ErrorRequestTimeout) LogLevel() logging.Level {
	return logging.INFO // Server timeouts are informational
}

// ErrorClientClosedRequest represents when client cancels the request.
type ErrorClientClosedRequest struct{}

func (ErrorClientClosedRequest) Error() string {
	return "client closed request"
}

func (ErrorClientClosedRequest) StatusCode() int {
	return StatusClientClosedRequest // Non-standard but widely used by Nginx
}

func (ErrorClientClosedRequest) LogLevel() logging.Level {
	return logging.DEBUG // Client cancellations aren't server errors
}

type ErrorServiceUnavailable struct {
	Dependency   string
	ErrorMessage string
}

func (e ErrorServiceUnavailable) Error() string {
	if e.ErrorMessage != "" && e.Dependency != "" {
		return fmt.Sprintf("Service unavailable due to error: %v from dependency %v", e.ErrorMessage, e.Dependency)
	}

	return http.StatusText(http.StatusServiceUnavailable)
}

func (ErrorServiceUnavailable) StatusCode() int {
	return http.StatusServiceUnavailable
}

func (ErrorServiceUnavailable) LogLevel() logging.Level {
	return logging.ERROR
}

// ErrorPanicRecovery represents an error for request which panicked.
type ErrorPanicRecovery struct{}

func (ErrorPanicRecovery) Error() string {
	return http.StatusText(http.StatusInternalServerError)
}

func (ErrorPanicRecovery) StatusCode() int {
	return http.StatusInternalServerError
}

func (ErrorPanicRecovery) LogLevel() logging.Level {
	return logging.ERROR
}

// validate the errors satisfy the underlying interfaces they depend on.
var (
	_ StatusCodeResponder = ErrorEntityNotFound{}
	_ StatusCodeResponder = ErrorEntityAlreadyExist{}
	_ StatusCodeResponder = ErrorInvalidParam{}
	_ StatusCodeResponder = ErrorMissingParam{}
	_ StatusCodeResponder = ErrorInvalidRoute{}
	_ StatusCodeResponder = ErrorRequestTimeout{}
	_ StatusCodeResponder = ErrorPanicRecovery{}
	_ StatusCodeResponder = ErrorServiceUnavailable{}
	_ StatusCodeResponder = ErrorClientClosedRequest{}

	_ logging.LogLevelResponder = ErrorClientClosedRequest{}
	_ logging.LogLevelResponder = ErrorEntityNotFound{}
	_ logging.LogLevelResponder = ErrorEntityAlreadyExist{}
	_ logging.LogLevelResponder = ErrorInvalidParam{}
	_ logging.LogLevelResponder = ErrorMissingParam{}
	_ logging.LogLevelResponder = ErrorInvalidRoute{}
	_ logging.LogLevelResponder = ErrorRequestTimeout{}
	_ logging.LogLevelResponder = ErrorPanicRecovery{}
	_ logging.LogLevelResponder = ErrorServiceUnavailable{}
)
