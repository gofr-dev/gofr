package error

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
)

func Test_ErrorGoFr(t *testing.T) {
	// with underlying error
	wrappedErr := New("underlying error")
	gofrErr := NewWrapped(wrappedErr, "custom message")

	expectedMsg := "custom message: underlying error"
	if !assert.Equal(t, gofrErr.Error(), expectedMsg) {
		t.Errorf("TestNewGofrError Failed")
	}

	// without underlying error
	gofrErr = New("custom message")
	expectedMsg = "custom message"

	if !assert.Equal(t, gofrErr.Error(), expectedMsg) {
		t.Errorf("TestNewGofrError Failed")
	}

	// without custom error message
	gofrErr = NewWrapped(wrappedErr)
	if !assert.Equal(t, "underlying error", gofrErr.Error()) {
		t.Errorf("TestNewGofrError Failed")
	}

	// without underlying error when WrappedError
	gofrErr = NewWrapped(nil, "custom message")
	if !assert.Equal(t, "custom message", gofrErr.Error()) {
		t.Errorf("TestNewGofrError Failed")
	}
}

func TestErrorGoFr_StatusCode(t *testing.T) {
	errGoFr := New("custom message")

	expectedCode := http.StatusInternalServerError
	if got := errGoFr.StatusCode(); got != expectedCode {
		t.Errorf("StatusCode(): expected %d, got %d", expectedCode, got)
	}
}
