package auth

import (
	"context"
	"fmt"
	"net/url"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/oauth2"
)

func TestNewOAuthConfig(t *testing.T) {
	testCases := []struct {
		name         string
		clientID     string
		clientSecret string
		tokenURL     string
		wantErr      bool
		errMsg       string
	}{
		{name: "empty client id", wantErr: true, errMsg: "client id is mandatory"},
		{name: "empty secret", clientID: "id", wantErr: true, errMsg: "client secret is mandatory"},
		{name: "empty token url", clientID: "id", clientSecret: "secret", wantErr: true,
			errMsg: "token url is mandatory"},
		{name: "invalid token url", clientID: "id", clientSecret: "secret", tokenURL: "invalid",
			wantErr: true, errMsg: "empty host"},
		{name: "valid config", clientID: "id", clientSecret: "secret",
			tokenURL: "https://auth.example.com/token"},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			opt, err := NewOAuthConfig(tc.clientID, tc.clientSecret, tc.tokenURL, nil, nil, oauth2.AuthStyleInParams)

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

func TestValidateTokenURL(t *testing.T) {
	testCases := []struct {
		name     string
		tokenURL string
		wantErr  bool
		errMsg   string
	}{
		{name: "valid https", tokenURL: "https://www.example.com"},
		{name: "valid http", tokenURL: "http://www.example.com"},
		{name: "host ends with dot", tokenURL: "https://www.example.", wantErr: true,
			errMsg: "invalid host pattern, ends with `.`"},
		{name: "host contains double dot", tokenURL: "https://www.192.168.1.1..com", wantErr: true,
			errMsg: "invalid host pattern, contains `..`"},
		{name: "non http scheme", tokenURL: "ftp://www.example.com", wantErr: true,
			errMsg: "invalid scheme, allowed http and https only"},
		{name: "empty url", wantErr: true, errMsg: "token url is mandatory"},
		{name: "missing host", tokenURL: "invalid_url_format", wantErr: true, errMsg: "empty host"},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			err := validateTokenURL(tc.tokenURL)

			if tc.wantErr {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tc.errMsg)

				return
			}

			assert.NoError(t, err)
		})
	}
}

func TestOAuthConfig_GetHeaderKey(t *testing.T) {
	cfg := &oAuthConfig{}
	assert.Equal(t, "Authorization", cfg.GetHeaderKey())
}

func TestOAuthConfig_GetHeaderValue(t *testing.T) {
	clientID, err := generateRandomString(clientIDLength)
	require.NoError(t, err)

	clientSecret, err := generateRandomString(clientSecretLength)
	require.NoError(t, err)

	server := setupOAuthHTTPServer(t, clientID, clientSecret, "test-audience")

	testCases := []struct {
		name         string
		clientID     string
		clientSecret string
		tokenURL     string
		wantErr      bool
	}{
		{
			name:         "valid credentials",
			clientID:     clientID,
			clientSecret: clientSecret,
			tokenURL:     server.URL + "/token",
		},
		{
			name:         "invalid credentials",
			clientID:     "wrong-id",
			clientSecret: "wrong-secret",
			tokenURL:     server.URL + "/token",
			wantErr:      true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			cfg := &oAuthConfig{
				clientID:     tc.clientID,
				clientSecret: tc.clientSecret,
				tokenURL:     tc.tokenURL,
				endpointParams: url.Values{
					"aud": {"test-audience"},
				},
				authStyle: oauth2.AuthStyleInParams,
			}

			value, err := cfg.GetHeaderValue(context.Background())

			if tc.wantErr {
				assert.Error(t, err)
				return
			}

			require.NoError(t, err)
			assert.True(t, strings.HasPrefix(value, "Bearer "),
				fmt.Sprintf("expected Bearer prefix, got: %s", value))
		})
	}
}
