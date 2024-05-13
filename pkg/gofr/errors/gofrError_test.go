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
	gofrErr := NewGofrError(wrappedErr, "custom message").WithStack()

	expectedMsg := fmt.Sprintf("custom message: %v", gofrErr.error)
	if !assert.Equal(t, gofrErr.Error(), expectedMsg) {
		t.Errorf("TestNewGofrError Failed")
	}

	// without underlying error
	gofrErr = NewGofrError(nil, "custom message")
	expectedMsg = "custom message"

	if !assert.Equal(t, gofrErr.Error(), expectedMsg) {
		t.Errorf("TestNewGofrError Failed")
	}
}
