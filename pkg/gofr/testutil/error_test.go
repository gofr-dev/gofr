package testutil

import (
	"github.com/stretchr/testify/assert"
	"testing"
)

func Test_CustomError(t *testing.T) {
	err := CustomError{ErrorMessage: "my error"}

	assert.Contains(t, err.ErrorMessage, "my error")
}
