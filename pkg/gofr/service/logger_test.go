package service

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestLog_PrettyPrint(t *testing.T) {
	w := new(bytes.Buffer)

	l := &Log{
		ResponseTime:  100,
		CorrelationID: "abc-test-correlation-id",
		ResponseCode:  200,
		HTTPMethod:    "GET",
		URI:           "/api/test",
	}

	l.PrettyPrint(w)

	assert.Equal(t, "\u001B[38;5;8mabc-test-correlation-id \u001B[38;5;34m200   \u001B[0m      100\u001B[38;5;8mµs\u001B[0m GET /api/test \n",
		w.String())
}

func TestErrorLog_PrettyPrint(t *testing.T) {
	w := new(bytes.Buffer)

	l := &ErrorLog{
		Log: &Log{
			ResponseTime:  100,
			CorrelationID: "abc-test-correlation-id",
			ResponseCode:  200,
			HTTPMethod:    "GET",
			URI:           "/api/test",
		},
		ErrorMessage: "some error occurred",
	}

	l.PrettyPrint(w)

	assert.Equal(t, "\u001B[38;5;8mabc-test-correlation-id \u001B[38;5;34m200   \u001B[0m      100\u001B[38;5;8mµs\u001B[0m GET /api/test \n",
		w.String())
}

func Test_ColorForStatusCode(t *testing.T) {
	testCases := []struct {
		desc   string
		code   int
		expOut int
	}{
		{desc: "200 OK", code: 200, expOut: 34},
		{desc: "201 Created", code: 201, expOut: 34},
		{desc: "400 Bad Request", code: 400, expOut: 220},
		{desc: "409 Conflict", code: 409, expOut: 220},
		{desc: "500 Internal Srv Error", code: 500, expOut: 202},
	}

	for _, tc := range testCases {
		out := colorForStatusCode(tc.code)

		assert.Equal(t, tc.expOut, out)
	}
}
