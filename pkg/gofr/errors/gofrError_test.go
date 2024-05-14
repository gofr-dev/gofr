package errors

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/pkg/errors"
)

func TestNewGofrError(t *testing.T) {
	// with underlying error
	wrappedErr := errors.New("underlying error")
	gofrErr := New(wrappedErr, "custom message")

	expectedMsg := fmt.Sprintf("custom message: %v", gofrErr.error)
	if !assert.Equal(t, gofrErr.Error(), expectedMsg) {
		t.Errorf("TestNewGofrError Failed")
	}

	// without underlying error
	gofrErr = New(nil, "custom message")
	expectedMsg = "custom message"

	if !assert.Equal(t, gofrErr.Error(), expectedMsg) {
		t.Errorf("TestNewGofrError Failed")
	}
}
