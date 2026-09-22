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

func TestCustomError_Error(t *testing.T) {
	tests := []struct {
		desc   string
		err    error
		expMsg string
	}{
		{desc: "message returned as error string", err: CustomError{ErrorMessage: "my error"}, expMsg: "my error"},
		{desc: "empty message", err: CustomError{}, expMsg: ""},
	}

	for _, tc := range tests {
		t.Run(tc.desc, func(t *testing.T) {
			assert.Equal(t, tc.expMsg, tc.err.Error())
		})
	}
}
