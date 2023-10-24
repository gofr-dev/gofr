package errors

import (
	"errors"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
)

type customError struct {
	errMsg string
}

func (e customError) Error() string {
	return e.errMsg
}

// Test_Raw to test Raw error type
func Test_Raw(t *testing.T) {
	testCases := []struct {
		desc       string
		statusCode int
		err        error
		expRes     string
	}{
		{desc: "In-built error provided", statusCode: http.StatusOK, err: errors.New("test error"), expRes: "test error"},
		{desc: "Custom error provided", statusCode: http.StatusOK, err: customError{"test error"}, expRes: "test error"},
		{desc: "Error not provided", statusCode: http.StatusOK, expRes: "Unknown Error"},
	}

	for i, tc := range testCases {
		err := Raw{StatusCode: tc.statusCode, Err: tc.err}

		assert.Equalf(t, tc.expRes, err.Error(), "Testcase [%d] Failed : %v", i+1, tc.desc)
	}
}
