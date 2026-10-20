package gofr

import (
	"net/http"
	"strings"

	"gofr.dev/pkg/gofr/config"
	gofrHTTP "gofr.dev/pkg/gofr/http"
	"gofr.dev/pkg/gofr/logging"
)

const hideInternalErrorsKey = "HTTP_HIDE_INTERNAL_ERRORS"

// readHideInternalErrors reports whether HTTP_HIDE_INTERNAL_ERRORS is set to true. Unset or false
// keeps error messages as they are; any other value is logged once and treated as false, so a typo
// never changes what clients receive.
func readHideInternalErrors(cfg config.Config, logger logging.Logger) bool {
	value := strings.TrimSpace(cfg.Get(hideInternalErrorsKey))

	switch {
	case value == "" || strings.EqualFold(value, "false"):
		return false
	case strings.EqualFold(value, "true"):
		return true
	default:
		logger.Warnf("invalid value of config %s: %q, expected true or false; error messages are not hidden",
			hideInternalErrorsKey, value)

		return false
	}
}

// hidesMessage reports whether err's message is replaced when HTTP_HIDE_INTERNAL_ERRORS is on: an
// error that declares no status code, or one that declares 500. Other statuses carry text written
// for clients, so they are kept. The direct type assertion matches the responder's own status code
// choice, so a hidden error is exactly one the client receives as a 500 (or 206 with data).
func hidesMessage(err error) bool {
	if err == nil {
		return false
	}

	e, ok := err.(gofrHTTP.StatusCodeResponder)

	return !ok || e.StatusCode() == http.StatusInternalServerError
}

// internalError stands in for an error whose message is hidden from the client. It keeps the 500
// status and any custom response fields of the original error; only the message changes.
type internalError struct {
	fields map[string]any
}

func newInternalError(err error) internalError {
	ie := internalError{}

	if rm, ok := err.(gofrHTTP.ResponseMarshaller); ok {
		ie.fields = rm.Response()
	}

	return ie
}

func (internalError) Error() string {
	return http.StatusText(http.StatusInternalServerError)
}

func (internalError) StatusCode() int {
	return http.StatusInternalServerError
}

func (e internalError) Response() map[string]any {
	return e.fields
}
