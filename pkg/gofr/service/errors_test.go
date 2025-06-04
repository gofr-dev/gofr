package service

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
)

var errTest = errors.New(`message inside error`)

func TestHttpService_OAuthError(t *testing.T) {
	testCases := []struct {
		err      error
		message  string
		response string
	}{
		{nil, "", "unknown error"},
		{nil, "error message", "error message"},
		{errTest, "", "message inside error"},
		{errTest, "error message", "error message\nmessage inside error"},
	}

	for i, tc := range testCases {
		oAuthError := OAuthErr{tc.err, tc.message}
		assert.Equal(t, tc.response, oAuthError.Error(), "failed test case #%d", i)
	}
}
