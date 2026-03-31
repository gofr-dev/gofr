package auth

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
)

var errTest = fmt.Errorf("message inside error")

func TestErr_Error(t *testing.T) {
	testCases := []struct {
		name     string
		err      error
		message  string
		expected string
	}{
		{name: "nil err and empty message", expected: "unknown error"},
		{name: "nil err with message", message: "error message", expected: "error message"},
		{name: "err with empty message", err: errTest, expected: "message inside error"},
		{name: "err with message", err: errTest, message: "error message",
			expected: fmt.Sprintf("%v: %v", "error message", errTest.Error())},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			authErr := Err{tc.err, tc.message}
			assert.Equal(t, tc.expected, authErr.Error())
		})
	}
}
