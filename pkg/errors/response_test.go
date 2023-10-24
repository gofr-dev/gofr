package errors

import (
	"errors"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestResponse_Error tests the behavior of Error method of Response type.
func TestResponse_Error(t *testing.T) {
	testCases := []struct {
		desc     string
		response *Response
		expError string
	}{
		{"Intenal Server Error", &Response{StatusCode: 500, Code: "UNKNOWN_ERROR",
			Reason: "unknown error occurred"}, "unknown error occurred"},
		{"Custom error provided", &Response{StatusCode: http.StatusBadRequest, Code: "test error",
			Reason: "test error"}, "test error"},
		{"Error with error Detail", &Response{StatusCode: 500, Code: "ERR_INTERNAL_SERVER_ERROR",
			Reason: "Internal Server Error", Detail: errors.New("database error")}, "Internal Server Error : database error "},
	}
	for i, tc := range testCases {
		err := Response{StatusCode: tc.response.StatusCode, Code: tc.response.Code, Reason: tc.response.Reason, Detail: tc.response.Detail}

		assert.Equal(t, tc.expError, err.Error(), "Test[%d] failed:%v", i, tc.desc)
	}
}

// TestResponse_Error tests the behavior of Error method of Error type.
func TestError_Error(t *testing.T) {
	var errorValue Error = "custom error"

	assert.Equal(t, "custom error", errorValue.Error(), "Test,failed:Custom Error")
}
