package middleware

import (
	"fmt"
	"net/http"
)

// ErrorHTTP represents an error specific to HTTP operations.
type ErrorHTTP interface {
	StatusCode() int
	error
}

// ErrorMissingAuthHeader represents the scenario where the auth header is missing from the request.
type ErrorMissingAuthHeader struct {
	key string
}

func NewMissingAuthHeaderError(key string) ErrorMissingAuthHeader {
	return ErrorMissingAuthHeader{key: key}
}

func (e ErrorMissingAuthHeader) Error() string {
	return fmt.Sprintf("missing auth header in key '%s'", e.key)
}

func (ErrorMissingAuthHeader) StatusCode() int {
	return http.StatusUnauthorized
}

// ErrorInvalidAuthorizationHeader represents the scenario where the auth header errMessage is invalid.
type ErrorInvalidAuthorizationHeader struct {
	key string
}

func NewInvalidAuthorizationHeaderError(key string) ErrorInvalidAuthorizationHeader {
	return ErrorInvalidAuthorizationHeader{key: key}
}
func (e ErrorInvalidAuthorizationHeader) Error() string {
	return fmt.Sprintf("invalid auth header in key '%s'", e.key)
}
func (ErrorInvalidAuthorizationHeader) StatusCode() int {
	return http.StatusUnauthorized
}

// ErrorInvalidAuthorizationHeaderFormat represents the scenario where the auth header errMessage is invalid.
type ErrorInvalidAuthorizationHeaderFormat struct {
	key        string
	errMessage string
}

func NewInvalidAuthorizationHeaderFormatError(key, format string) ErrorInvalidAuthorizationHeaderFormat {
	return ErrorInvalidAuthorizationHeaderFormat{key: key, errMessage: format}
}
func (e ErrorInvalidAuthorizationHeaderFormat) Error() string {
	return fmt.Sprintf("invalid value in '%s' header - %s", e.key, e.errMessage)
}
func (ErrorInvalidAuthorizationHeaderFormat) StatusCode() int {
	return http.StatusUnauthorized
}

type ErrorForbidden struct {
	message string
}

func NewUnauthorized(message string) ErrorForbidden {
	return ErrorForbidden{message: message}
}
func (e ErrorForbidden) Error() string {
	if e.message != "" {
		return e.message
	}

	return http.StatusText(http.StatusForbidden)
}
func (ErrorForbidden) StatusCode() int {
	return http.StatusForbidden
}

type Field struct {
	key    string
	format string
}
type ErrorBadRequest struct {
	fields []Field
}

func NewBadRequest(fields []Field) ErrorBadRequest {
	return ErrorBadRequest{fields: fields}
}
func (e ErrorBadRequest) Error() string {
	return fmt.Sprintf("bad request, invalid value in %d fields", len(e.fields))
}
func (ErrorBadRequest) StatusCode() int {
	return http.StatusBadRequest
}

type ErrorInvalidConfiguration struct {
	message string
}

func NewInvalidConfigurationError(message string) ErrorInvalidConfiguration {
	return ErrorInvalidConfiguration{message: message}
}
func (e ErrorInvalidConfiguration) Error() string {
	return fmt.Sprintf("invalid configuration %s - please contact administrator", e.message)
}

func (ErrorInvalidConfiguration) StatusCode() int {
	return http.StatusInternalServerError
}

// ErrorRequestBodyTooLarge represents the scenario where the request body exceeds the maximum allowed size.
type ErrorRequestBodyTooLarge struct {
	maxSize int64
	actual  int64
}

func NewRequestBodyTooLargeError(maxSize, actual int64) ErrorRequestBodyTooLarge {
	return ErrorRequestBodyTooLarge{maxSize: maxSize, actual: actual}
}

func (e ErrorRequestBodyTooLarge) Error() string {
	return fmt.Sprintf("request body too large: %d bytes exceeds maximum allowed size of %d bytes", e.actual, e.maxSize)
}

func (ErrorRequestBodyTooLarge) StatusCode() int {
	return http.StatusRequestEntityTooLarge
}
