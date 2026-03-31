package auth

import (
	"context"
	"encoding/base64"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"gofr.dev/pkg/gofr/service"
)

func TestNewBasicAuthConfig(t *testing.T) {
	testCases := []struct {
		name     string
		username string
		password string
		wantErr  bool
		errMsg   string
	}{
		{name: "empty username", password: "cGFzc3dvcmQ=",
			wantErr: true, errMsg: "username is required"},
		{name: "whitespace username", username: "  ",
			password: "cGFzc3dvcmQ=", wantErr: true, errMsg: "username is required"},
		{name: "empty password", username: "user",
			wantErr: true, errMsg: "password is required"},
		{name: "invalid base64 password", username: "user",
			password: "cGFzc3dvcmQ===", wantErr: true,
			errMsg: "password should be base64 encoded"},
		{name: "non-encoded password", username: "user", password: "plaintext",
			wantErr: true, errMsg: "password should be base64 encoded"},
		{name: "valid credentials", username: "user", password: "cGFzc3dvcmQ="},
		{name: "trimmed whitespace", username: "  user  ", password: "  cGFzc3dvcmQ="},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			opt, err := NewBasicAuthConfig(tc.username, tc.password)

			if tc.wantErr {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tc.errMsg)
				assert.Nil(t, opt)

				return
			}

			require.NoError(t, err)
			assert.NotNil(t, opt)
		})
	}
}

func TestBasicAuthConfig_GetHeaderKey(t *testing.T) {
	cfg := &basicAuthConfig{userName: "user", password: "pass"}
	assert.Equal(t, service.AuthHeader, cfg.GetHeaderKey())
}

func TestBasicAuthConfig_GetHeaderValue(t *testing.T) {
	cfg := &basicAuthConfig{
		userName:    "user",
		password:    "password",
		headerValue: "Basic " + base64.StdEncoding.EncodeToString([]byte("user:password")),
	}

	value, err := cfg.GetHeaderValue(context.Background())
	require.NoError(t, err)
	assert.Equal(t, "Basic "+base64.StdEncoding.EncodeToString([]byte("user:password")), value)
}
