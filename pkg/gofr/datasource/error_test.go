package datasource

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/pkg/errors"
)

func Test_ErrorDB(t *testing.T) {
	// Test with wrapped error
	wrappedErr := errors.New("underlying error")
	dbErr := ErrorWrapped(wrappedErr, "custom message").WithStack()

	expectedMsg := "custom message: underlying error"
	if !assert.Equal(t, dbErr.Error(), expectedMsg) {
		t.Errorf("Test_ErrorDB wrapped error failed")
	}

	// Test with no wrapped error
	dbErr = Error("custom message")

	expectedMsg = "custom message"
	if !assert.Equal(t, dbErr.Error(), expectedMsg) {
		t.Errorf("Test_ErrorDB no wrapped error dailed")
	}

	// Test without custom error message
	dbErr = ErrorWrapped(wrappedErr)
	if !assert.Equal(t, "underlying error", dbErr.Error()) {
		t.Errorf("Test_ErrorDB without custom error message failed")
	}

	// without underlying error when WrappedError
	dbErr = ErrorWrapped(nil, "custom message")
	if !assert.Equal(t, "custom message", dbErr.Error()) {
		t.Errorf("Test_ErrorDB without underlying error Failed")
	}
}

func TestErrorDB_StatusCode(t *testing.T) {
	dbErr := Error("custom message")

	expectedCode := http.StatusInternalServerError
	if got := dbErr.StatusCode(); got != expectedCode {
		t.Errorf("StatusCode(): expected %d, got %d", expectedCode, got)
	}
}
