package serrors

import (
	"errors"
	"reflect"
	"testing"
)

func TestErrorStructFields(t *testing.T) {
	metaData := map[string]any{
		"requestID": "abc123",
		"timestamp": 1650000000,
		"nested":    map[string]any{"inner": "value"},
	}

	errCause := errors.New("underlying error")

	err := Error{
		cause:              errCause,
		message:            "Top-level message",
		statusCode:         "E100",
		subStatusCode:      "E101",
		level:              Level(3),
		meta:               metaData,
		retryable:          true,
		externalStatusCode: 503,
		externalMessage:    "Service Unavailable",
	}

	// Field assertions
	if err.cause.Error() != "underlying error" {
		t.Errorf("expected cause to be 'underlying error', got %v", err.cause)
	}
	if err.message != "Top-level message" {
		t.Errorf("expected message to be 'Top-level message', got %s", err.message)
	}
	if err.statusCode != "E100" {
		t.Errorf("expected statusCode to be 'E100', got %s", err.statusCode)
	}
	if err.subStatusCode != "E101" {
		t.Errorf("expected subStatusCode to be 'E101', got %s", err.subStatusCode)
	}
	if err.level.GetErrorLevel() != "ERROR" {
		t.Errorf("expected level to be 'ERROR', got %s", err.level.GetErrorLevel())
	}
	if !reflect.DeepEqual(err.meta, metaData) {
		t.Errorf("expected meta to match, got %+v", err.meta)
	}
	if !err.retryable {
		t.Errorf("expected retryable to be true, got %v", err.retryable)
	}
	if err.externalStatusCode != 503 {
		t.Errorf("expected externalStatusCode to be 503, got %d", err.externalStatusCode)
	}
	if err.externalMessage != "Service Unavailable" {
		t.Errorf("expected externalMessage to be 'Service Unavailable', got %s", err.externalMessage)
	}
}
