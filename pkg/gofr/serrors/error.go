package serrors

import (
	"fmt"
	"time"
)

const UNSET = "NA"

// New creates a new Error instance by wrapping an existing error with additional context.
// It captures the current timestamp in nanoseconds for precise error tracing and sets default values
// for error metadata. The resulting error maintains the original error chain while providing a new
// contextual message.
//
// Parameters:
//
//	err     : The original error being wrapped (can be nil). Preserved for error chain inspection.
//	message : Human-readable context message describing the error situation. Should provide
//	          higher-level context about where/why the error occurred.
//
// Returns:
//
//	*Error : Structured error instance containing:
//	         - Wrapped cause (original error)
//	         - Provided contextual message
//	         - Timestamp in UTC nanoseconds (meta.timestamp)
//	         - Default metadata:
//	             statusCode: "NA" (unclassified status)
//	             subStatusCode: "NA" (unclassified sub-status)
//	             level: ERROR (default severity)
//	             retryable: false (default non-retryable)
//	             externalStatusCode: 0 (unset external status)
//	             externalMessage: "NA" (unset client message)
//
// Usage:
//
//	if err := io.Read(file); err != nil {
//	    return errors.Wrap(err, "config load failed")
//	}
func New(err error, message string) *Error {
	meta := make(map[string]any)

	// Capture precise timestamp for error tracing
	meta["timestamp"] = fmt.Sprintf("%d", (time.Now().UTC()).UnixNano())

	return &Error{
		cause:              err,     // Preserve original error
		message:            message, // Add contextual message
		statusCode:         UNSET,   // Default unclassified status
		subStatusCode:      UNSET,   // Default unclassified sub-status
		level:              ERROR,   // Default error severity
		retryable:          false,   // Default non retryable
		meta:               meta,    // Metadata with timestamp
		externalStatusCode: 0,       // Unset external status code
		externalMessage:    UNSET,   // Unset external message
	}
}

// GetInternalError used when logging errors
func GetInternalError(err *Error, addMeta bool) string {
	var errorCause string

	if err.cause == nil {
		errorCause = "Nil cause"
	}

	msg := fmt.Sprintf(
		"%s | %s | %s | %s | %s",
		err.level.GetErrorLevel(),
		err.statusCode,
		err.subStatusCode,
		err.message,
		errorCause,
	)

	if addMeta {
		msg += fmt.Sprintf("| %s", getMetaString(err.meta))
	}

	return msg
}

// GetExternalError formats a standardized client-facing error message from an Error instance.
// This formatted string is suitable for returning to API consumers and includes only
// the external-facing error information. The output format is consistent for machine parsing:
//
//	"[status_code] | [client_message]"
//
// Parameters:
//
//	err : Pointer to Error struct - must not be nil (caller responsibility to check)
//
// Returns:
//
//	string : Formatted error message containing:
//	         - externalStatusCode: Numeric status code (e.g., HTTP 404)
//	         - externalMessage: Safe message for client consumption
//
// Behavior:
//   - If externalStatusCode is 0 (unset), outputs "0 | [message]"
//   - If externalMessage is empty, outputs "[status] | " (empty message)
//   - Handles all integer status codes including negative values
//   - Returns "0 | NA" if passed a nil pointer (safe fallback)
//
// Example outputs:
//
//	"404 | Resource not found"
//	"401 | Invalid authentication token"
//	"0 | NA" (when passed nil)
//
// Usage:
//
//	apiErr := error.Wrap(nil, "User not found")
//	clientMsg := errors.GetExternalError(apiErr)
//	// clientMsg = "404 | User not found"
func GetExternalError(err *Error) string {
	// Safe handling for nil pointer
	if err == nil {
		return "0 | NA"
	}

	// Format standardized error string
	msg := fmt.Sprintf(
		"%d | %s",
		err.externalStatusCode,
		err.externalMessage)

	return msg
}
