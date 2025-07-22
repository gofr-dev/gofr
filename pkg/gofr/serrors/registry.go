package serrors

import (
	"fmt"
)

type Registry struct {
	InternalStatus  int
	InternalMessage string
	ExternalStatus  int
	ExternalMessage string
	Level           Level
	SubStatusCode   string
	Retryable       bool
}

func NewFromRegistry(err error, statusCode string, registry map[string]Registry) *Error {
	entry, ok := registry[statusCode]
	if !ok {
		return New(err, fmt.Sprintf("Unknown status code %s", statusCode))
	}
	sError := New(err, entry.InternalMessage)
	sError.WithStatusCode(statusCode)
	sError.WithExternalStatus(entry.ExternalStatus)
	sError.WithExternalMessage(entry.ExternalMessage)
	sError.WithLevel(entry.Level)
	sError.WithSubCode(entry.SubStatusCode)
	sError.WithRetryable(entry.Retryable)

	return sError
}
