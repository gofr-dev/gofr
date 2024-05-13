package errors

import (
	"fmt"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/pkg/errors"
)

func TestNewDBError(t *testing.T) {
	// Test with wrapped error
	wrappedErr := errors.New("underlying error")
	dbErr := NewDBError(wrappedErr, "custom message")

	expectedMsg := fmt.Sprintf("custom message: %v", dbErr.error)
	if !assert.Equal(t, dbErr.Error(), expectedMsg) {
		t.Errorf("TestNewGofrError Failed")
	}

	// Test with no wrapped error
	dbErr = NewDBError(nil, "custom message")

	expectedMsg = "custom message"
	if !assert.Equal(t, dbErr.Error(), expectedMsg) {
		t.Errorf("TestNewGofrError Failed")
	}
}

func TestDBError_StatusCode(t *testing.T) {
	dbErr := NewDBError(nil, "custom message").WithStack()

	expectedCode := http.StatusInternalServerError
	if got := dbErr.StatusCode(); got != expectedCode {
		t.Errorf("StatusCode(): expected %d, got %d", expectedCode, got)
	}
}
