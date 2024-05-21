package error

import (
	"errors"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
)

var (
	errTest = errors.New("underlying error")
)

func Test_ErrorGoFr(t *testing.T) {
	// with underlying error
	gofrErr := ErrGoFr{Err: errTest, Message: "custom message"}.WithStack()

	expectedMsg := "custom message: underlying error"
	if !assert.Equal(t, gofrErr.Error(), expectedMsg) {
		t.Errorf("TestNewGofrError Failed")
	}

	// without underlying error
	gofrErr = ErrGoFr{Message: "custom message"}
	expectedMsg = "custom message"

	if !assert.Equal(t, gofrErr.Error(), expectedMsg) {
		t.Errorf("TestNewGofrError Failed")
	}

	// without custom error message
	gofrErr = ErrGoFr{Err: errTest}.WithStack()
	if !assert.Equal(t, "underlying error", gofrErr.Error()) {
		t.Errorf("TestNewGofrError Failed")
	}

	// without underlying error when WrappedError
	gofrErr = ErrGoFr{Message: "custom message"}
	if !assert.Equal(t, "custom message", gofrErr.Error()) {
		t.Errorf("TestNewGofrError Failed")
	}
}

func TestErrorGoFr_StatusCode(t *testing.T) {
	errGoFr := ErrGoFr{Message: "custom message"}

	expectedCode := http.StatusInternalServerError
	if got := errGoFr.StatusCode(); got != expectedCode {
		t.Errorf("StatusCode(): expected %d, got %d", expectedCode, got)
	}
}
