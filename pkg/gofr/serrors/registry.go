package serrors

import (
	"fmt"
)

type Registry struct {
	InternalStatus  int    // Internal numeric status code used within the application
	InternalMessage string // Human-readable internal error message.
	ExternalStatus  int    // Status code intended for external consumers (e.g., HTTP status).
	ExternalMessage string // User-facing error message meant for external communication.
	Level           Level  // Severity level of the error (e.g., info, warning, error).
	SubStatusCode   string // Additional code for more granular classification of the error.
	Retryable       bool   // Indicates whether the operation that caused the error can be safely retried.
}

// NewFromRegistry constructs a custom *Error object based on a given status code and a registry of predefined error entries.
//
// Parameters:
// - err:         The original error that triggered this handling.
// - statusCode:  A string identifier representing the error type or status.
// - registry:    A map where keys are status codes and values are Registry entries containing metadata for each error.
//
// Behavior:
//   - If the statusCode exists in the registry, the function creates a new *Error using the associated internal message
//     and populates it with additional metadata from the registry entry (e.g., external status, message, level, sub-status code, retryable flag).
//   - If the statusCode is not found in the registry, it returns a generic *Error indicating the status code is unknown.
//
// Returns:
// - A fully populated *Error object appropriate to the context provided.
func NewFromRegistry(err error, statusCode string, registry map[string]Registry) *Error {
	entry, ok := registry[statusCode]
	if !ok {
		return New(err, fmt.Sprintf("Unknown status code %s", statusCode))
	}

	sError := New(err, entry.InternalMessage)

	sError.
		WithStatusCode(statusCode).
		WithExternalStatus(entry.ExternalStatus).
		WithExternalMessage(entry.ExternalMessage).
		WithLevel(entry.Level).
		WithSubCode(entry.SubStatusCode).
		WithRetryable(entry.Retryable)

	return sError
}
