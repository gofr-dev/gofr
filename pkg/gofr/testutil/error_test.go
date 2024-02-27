package testutil

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func Test_CustomError(t *testing.T) {
	err := CustomError{ErrorMessage: "my error"}

	assert.Contains(t, err.ErrorMessage, "my error")
}
