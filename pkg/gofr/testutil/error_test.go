package testutil

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestMain(m *testing.M) {
	os.Setenv("GOFR_TELEMETRY", "false")
	m.Run()
}

func Test_CustomError(t *testing.T) {
	err := CustomError{ErrorMessage: "my error"}

	assert.Contains(t, err.ErrorMessage, "my error")
}
