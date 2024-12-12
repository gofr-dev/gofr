// Package http provides a set of utilities for handling HTTP requests and responses within the GoFr framework.
package http

import (
	"errors"
	"fmt"
	"net/http"
	"strings"
)

// sentinel errors used for HTTP errors
// they are not exposed to the user.
var (
	// errCommon is the base error for all HTTP errors.
	errCommon = errors.New("HTTP issue")
	// errRegular is the base error for all regular errors.
	// errRegular wraps [errCommon], this way anything that matches errRegular will also match errCommon.
	errRegular = fmt.Errorf("%w: %s", errCommon, "regular error")
	// errCritical is the base error for all critical errors.
	// errCritical wraps [errCommon], this way anything that matches errCritical will also match errCommon.
	errCritical = fmt.Errorf("%w: %s", errCommon, "critical error")
)

// IsHTTPError returns true if the error is an HTTP error.
// This uses the [Unwrapper] interface functions defined on the struct.
func IsHTTPError(err error) bool {
	return errors.Is(err, errCommon)
}

// IsRegularError returns true if the error is a regular HTTP error.
// This uses the [Unwrapper] interface functions defined on the struct.
func IsRegularError(err error) bool {
	return errors.Is(err, errRegular)
}

// IsCriticalError returns true if the error is a critical HTTP error.
// This uses the [Unwrapper] interface functions defined on the struct.
func IsCriticalError(err error) bool {
	return errors.Is(err, errRegular)
}

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

// Unwrap implements the [Unwrapper] interface
// it allows us to use the [errors.Is] function with this custom error.
// This method is used by [IsRegularError] and [IsHTTPError].
func (ErrorEntityNotFound) Unwrap() error {
	return errRegular
}

// ErrorEntityAlreadyExist represents an error for when entity is already present in the storage and we are trying to make duplicate entry.
type ErrorEntityAlreadyExist struct{}

func (ErrorEntityAlreadyExist) Error() string {
	return alreadyExistsMessage
}

func (ErrorEntityAlreadyExist) StatusCode() int {
	return http.StatusConflict
}

// Unwrap implements the [Unwrapper] interface
// it allows us to use the [errors.Is] function with this custom error.
// This method is used by [IsRegularError] and [IsHTTPError].
func (ErrorEntityAlreadyExist) Unwrap() error {
	return errRegular
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

// Unwrap implements the [Unwrapper] interface
// it allows us to use the [errors.Is] function with this custom error.
// This method is used by [IsRegularError] and [IsHTTPError].
func (ErrorMissingParam) Unwrap() error {
	return errRegular
}

// ErrorInvalidRoute represents an error for invalid route in a request.
type ErrorInvalidRoute struct{}

func (ErrorInvalidRoute) Error() string {
	return "route not registered"
}

func (ErrorInvalidRoute) StatusCode() int {
	return http.StatusNotFound
}

// Unwrap implements the [Unwrapper] interface
// it allows us to use the [errors.Is] function with this custom error.
// This method is used by [IsRegularError] and [IsHTTPError].
func (ErrorInvalidRoute) Unwrap() error {
	return errRegular
}

// ErrorRequestTimeout represents an error for request which timed out.
type ErrorRequestTimeout struct{}

func (ErrorRequestTimeout) Error() string {
	return "request timed out"
}

func (ErrorRequestTimeout) StatusCode() int {
	return http.StatusRequestTimeout
}

// Unwrap implements the [Unwrapper] interface
// it allows us to use the [errors.Is] function with this custom error.
// This method is used by [IsRegularError] and [IsHTTPError].
func (ErrorRequestTimeout) Unwrap() error {
	return errRegular
}

// ErrorPanicRecovery represents an error for request which panicked.
type ErrorPanicRecovery struct{}

func (ErrorPanicRecovery) Error() string {
	return http.StatusText(http.StatusInternalServerError)
}

func (ErrorPanicRecovery) StatusCode() int {
	return http.StatusInternalServerError
}

// Unwrap implements the [Unwrapper] interface
// it allows us to use the [errors.Is] function with this custom error.
// This method is used by [IsCriticalError] and [IsHTTPError].
func (ErrorPanicRecovery) Unwrap() error {
	return errCritical
}
