package serrors

import (
	"errors"
	"fmt"
	"testing"
)

var errBase = errors.New("base")

func TestNewFromRegistry_Positive(t *testing.T) {
	errBase = errors.New("base error") //nolint:err113 // error pkg tests
	statusCode := "E100"

	reg := map[string]Registry{
		statusCode: {
			InternalStatus:  500,
			InternalMessage: "Internal Failure",
			ExternalStatus:  503,
			ExternalMessage: "Service Unavailable",
			Level:           ERROR,
			SubStatusCode:   "E101",
			Retryable:       true,
		},
	}

	result := NewFromRegistry(errBase, statusCode, reg)

	if result == nil {
		t.Fatal("Expected non-nil Error")
	}

	if result.message != "Internal Failure" {
		t.Errorf("Expected message 'Internal Failure', got '%s'", result.message)
	}

	if result.statusCode != statusCode {
		t.Errorf("Expected statusCode '%s', got '%s'", statusCode, result.statusCode)
	}

	if result.externalStatusCode != 503 {
		t.Errorf("Expected externalStatusCode 503, got %d", result.externalStatusCode)
	}

	if result.externalMessage != "Service Unavailable" {
		t.Errorf("Expected externalMessage 'Service Unavailable', got '%s'", result.externalMessage)
	}

	if result.level.GetErrorLevel() != "ERROR" {
		t.Errorf("Expected level 'ERROR', got '%s'", result.level.GetErrorLevel())
	}

	if result.subStatusCode != "E101" {
		t.Errorf("Expected subStatusCode 'E101', got '%s'", result.subStatusCode)
	}

	if !result.retryable {
		t.Errorf("Expected retryable true, got %v", result.retryable)
	}
}

func TestNewFromRegistry_Negative(t *testing.T) {
	errBase = errors.New("base error") //nolint:err113 // error pkg tests
	statusCode := "UNKNOWN"

	reg := map[string]Registry{
		"E100": {
			InternalMessage: "some message",
		},
	}

	result := NewFromRegistry(errBase, statusCode, reg)

	if result == nil {
		t.Fatal("Expected non-nil Error")
	}

	expectedMessage := fmt.Sprintf("Unknown status code %s", statusCode)
	if result.message != expectedMessage {
		t.Errorf("Expected message '%s', got '%s'", expectedMessage, result.message)
	}

	if result.statusCode != UNSET {
		t.Errorf("Expected empty statusCode, got '%s'", result.statusCode)
	}

	if result.externalStatusCode != 0 {
		t.Errorf("Expected default externalStatusCode 0, got %d", result.externalStatusCode)
	}

	if result.level.GetErrorLevel() != "UNKNOWN" {
		t.Errorf("Expected UNKNOWN level, got %+v", result.level)
	}
}
