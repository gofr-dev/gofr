package service

import (
	"encoding/base64"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"
)

func TestNewBasicAuthConfig(t *testing.T) {
	badPasswordErr := AuthErr{Err: base64.CorruptInputError(12), Message: "password should be base64 encoded"}
	testCases := []struct {
		username string
		password string
		option   Options
		err      error
	}{
		{username: "value", password: "", option: nil, err: AuthErr{Message: "password is required"}},
		{username: "", password: "", option: nil, err: AuthErr{Message: "username is required"}},
		{username: "  ", password: "", option: nil, err: AuthErr{Message: "username is required"}},
		{username: "value", password: "cGFzc3dvcmQ===", option: nil, err: badPasswordErr},
		{username: "value", password: "cGFzc3dvcmQ=", option: &BasicAuthConfig{"value", "password"}, err: nil},
		{username: "  value ", password: "cGFzc3dvcmQ=", option: &BasicAuthConfig{"value", "password"}, err: nil},
		{username: "  value ", password: "  cGFzc3dvcmQ=", option: &BasicAuthConfig{"value", "password"}, err: nil},
	}

	for i, tc := range testCases {
		result, err := NewBasicAuthConfig(tc.username, tc.password)
		assert.Equal(t, tc.option, result, "failed test case #%d", i)
		assert.Equal(t, tc.err, err, "failed test case #%d", i)
	}
}

func TestAddAuthorizationHeader_BasicAuth(t *testing.T) {

	testCases := []struct {
		username string
		password string
		headers  map[string]string
		response map[string]string
		err      error
	}{
		{
			username: "username",
			password: "password",
			headers:  nil,
			response: map[string]string{AuthHeader: "basic dXNlcm5hbWU6cGFzc3dvcmQ="},
		},
		{
			username: "username",
			password: "password",
			headers:  map[string]string{AuthHeader: "existing value"},
			response: map[string]string{AuthHeader: "existing value"},
			err:      AuthErr{Message: "value existing value already exists for header Authorization"},
		},
		{
			username: "username",
			password: "password",
			headers:  map[string]string{"header-key": "existing-value"},
			response: map[string]string{"header-key": "existing-value", AuthHeader: "basic dXNlcm5hbWU6cGFzc3dvcmQ="},
			err:      nil,
		},
	}
	for i, tc := range testCases {
		authProvider := basicAuthProvider{userName: tc.username, password: tc.password}
		response, err := authProvider.addAuthorizationHeader(t.Context(), tc.headers)
		assert.Equal(t, tc.response, response, "failed test case #%d", i)
		assert.Equal(t, tc.err, err, "failed test case #%d", i)
	}
}

func TestBasicAuthProvider(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	// separate mock servers having their own validations
}

func setupBasicAuthHTTPServer(t *testing.T, apiKey, headerAPIKey string, responseCode int) *httptest.Server {
	t.Helper()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// TODO add assertion for valid credentials being passed

		// TODO add check for valid credentials and update responseCode

		w.WriteHeader(responseCode)
	}))

	t.Cleanup(func() {
		server.Close()
	})

	return server
}
