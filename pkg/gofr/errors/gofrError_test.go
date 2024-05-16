package gofrerror

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/pkg/errors"
)

func TestNewGofrError(t *testing.T) {
	// with underlying error
	wrappedErr := errors.New("underlying error")
	gofrErr := NewError(wrappedErr, "custom message")

	expectedMsg := "custom message: underlying error"
	if !assert.Equal(t, gofrErr.Error(), expectedMsg) {
		t.Errorf("TestNewGofrError Failed")
	}

	// without underlying error
	gofrErr = NewError(nil, "custom message", "custom message 2")
	expectedMsg = "custom message custom message 2"

	if !assert.Equal(t, gofrErr.Error(), expectedMsg) {
		t.Errorf("TestNewGofrError Failed")
	}

	// without custom error message
	gofrErr = NewError(wrappedErr)
	if !assert.Equal(t, "underlying error", gofrErr.Error()) {
		t.Errorf("TestNewGofrError Failed")
	}
}

func TestErrorGoFr_StatusCode(t *testing.T) {
	errGoFr := NewError(nil, "custom message")

	expectedCode := http.StatusInternalServerError
	if got := errGoFr.StatusCode(); got != expectedCode {
		t.Errorf("StatusCode(): expected %d, got %d", expectedCode, got)
	}
}
