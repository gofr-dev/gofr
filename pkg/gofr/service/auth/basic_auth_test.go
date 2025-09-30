package auth

import (
	"encoding/base64"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"

	"gofr.dev/pkg/gofr/service"
)

func TestNewBasicAuthConfig(t *testing.T) {
	badPasswordErr := service.AuthErr{Err: base64.CorruptInputError(12), Message: "password should be base64 encoded"}
	testCases := []struct {
		username string
		password string
		option   service.Options
		err      error
	}{
		{username: "value", password: "", option: nil, err: service.AuthErr{Message: "password is required"}},
		{username: "", password: "", option: nil, err: service.AuthErr{Message: "username is required"}},
		{username: "  ", password: "", option: nil, err: service.AuthErr{Message: "username is required"}},
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
			password: "cGFzc3dvcmQ=",
			headers:  nil,
			response: map[string]string{AuthHeader: "basic dXNlcm5hbWU6cGFzc3dvcmQ="},
		},
		{
			username: "username",
			password: "cGFzc3dvcmQ=",
			headers:  map[string]string{AuthHeader: "existing value"},
			response: map[string]string{AuthHeader: "existing value"},
			err:      service.AuthErr{Message: "value existing value already exists for header Authorization"},
		},
		{
			username: "username",
			password: "cGFzc3dvcmQ=",
			headers:  map[string]string{"header-key": "existing-value"},
			response: map[string]string{"header-key": "existing-value", AuthHeader: "basic dXNlcm5hbWU6cGFzc3dvcmQ="},
			err:      nil,
		},
	}
	for i, tc := range testCases {
		config, err := NewBasicAuthConfig(tc.username, tc.password)
		if err != nil {
			t.Fatalf("unable to get basicAuthConfig for test case #%d", i)
		}

		basicAuthConfig, ok := config.(*BasicAuthConfig)
		if !ok {
			t.Fatalf("unable to get basicAuthConfig for test case #%d", i)
		}

		response, err := basicAuthConfig.addAuthorizationHeader(t.Context(), tc.headers)
		assert.Equal(t, tc.response, response, "failed test case #%d", i)
		assert.Equal(t, tc.err, err, "failed test case #%d", i)
	}
}

func setupBasicAuthHTTPServer(t *testing.T, config *BasicAuthConfig) *httptest.Server {
	t.Helper()

	validHeader := "basic " + base64.StdEncoding.EncodeToString([]byte(config.UserName+":"+config.Password))
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		statusCode := http.StatusOK
		if r.Header.Get(AuthHeader) != validHeader {
			statusCode = http.StatusUnauthorized
		}

		w.WriteHeader(statusCode)
	}))

	t.Cleanup(func() {
		server.Close()
	})

	return server
}
