package middleware

import (
	"fmt"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestHTTPError(t *testing.T) {
	testCases := []struct {
		err        ErrorHTTP
		statusCode int
		message    string
	}{
		{
			err:        ErrorMissingAuthHeader{key: "X-Api-Key"},
			statusCode: http.StatusUnauthorized,
			message:    "missing auth header in key 'X-Api-Key'",
		},
		{
			err:        ErrorInvalidAuthorizationHeaderFormat{key: "Authorization", errMessage: "Bearer {value}"},
			statusCode: http.StatusUnauthorized,
			message:    "invalid value in 'Authorization' header - Bearer {value}",
		},
		{
			err:        ErrorForbidden{message: "operation forbidden"},
			statusCode: http.StatusForbidden,
			message:    "operation forbidden",
		},
		{
			err:        ErrorForbidden{},
			statusCode: http.StatusForbidden,
			message:    "Forbidden",
		},
		{
			err:        ErrorBadRequest{fields: []Field{{key: "name", format: "uppercase"}, {key: "value", format: "uppercase"}}},
			statusCode: http.StatusBadRequest,
			message:    "bad request, invalid value in 2 fields",
		},
	}

	for i, tc := range testCases {
		t.Run(fmt.Sprintf("Test Case #%d", i), func(t *testing.T) {
			assert.Equal(t, tc.statusCode, tc.err.StatusCode())
			assert.Equal(t, tc.message, tc.err.Error())
		})
	}
}
