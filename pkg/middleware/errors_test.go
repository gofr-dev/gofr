package middleware

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func Test_GetDescription(t *testing.T) {
	tests := []struct {
		desc    string
		input   error
		expDesc string
		expOut  int
	}{
		{"invalid token", ErrInvalidToken, "The access token is invalid or has expired", 401},
		{"invalid Request", ErrInvalidRequest, "The access token is missing", 401},
		{"Service Unavailable", ErrServiceDown, "Unable to validate the token", 500},
		{"missing Header", ErrMissingHeader, "Missing Authorization header", 401},
		{"invalid Header", ErrInvalidHeader, "Invalid Authorization header", 400},
		{"unauthorized", ErrUnauthorised, "Authorization error", 403},
		{"Unauthenticated", ErrUnauthenticated, "Authorization error", 401},
	}
	for i, tc := range tests {
		desc, output := GetDescription(tc.input)

		assert.Equal(t, tc.expDesc, desc, "Test Failed %v", i)
		assert.Equal(t, tc.expOut, output, "Test Failed %v", i)
	}
}
func Test_Error(t *testing.T) {
	output := ErrInvalidRequest.Error()

	assert.Equal(t, "invalid_request", output, "Test Failed")
}
