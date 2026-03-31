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
	"golang.org/x/oauth2/clientcredentials"

	"gofr.dev/pkg/gofr/service"
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
		{name: "empty client id", wantErr: true, errMsg: "client id is required"},
		{name: "empty secret", clientID: "id", wantErr: true, errMsg: "client secret is required"},
		{name: "empty token url", clientID: "id", clientSecret: "secret", wantErr: true,
			errMsg: "token url is required"},
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
		{name: "empty url", wantErr: true, errMsg: "token url is required"},
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

func TestOAuthTokenSource_GetHeaderKey(t *testing.T) {
	provider := &bearerAuthProvider{source: &oAuthTokenSource{}}
	assert.Equal(t, service.AuthHeader, provider.GetHeaderKey())
}

func TestOAuthTokenSource_Token(t *testing.T) {
	clientID, err := generateRandomString(clientIDLength)
	require.NoError(t, err)

	clientSecret, err := generateRandomString(clientSecretLength)
	require.NoError(t, err)

	server := setupOAuthHTTPServer(t, clientID, clientSecret, "test-audience")

	testCases := []struct {
		name    string
		config  clientcredentials.Config
		wantErr bool
	}{
		{
			name: "valid credentials",
			config: clientcredentials.Config{
				ClientID:       clientID,
				ClientSecret:   clientSecret,
				TokenURL:       server.URL + "/token",
				EndpointParams: url.Values{"aud": {"test-audience"}},
				AuthStyle:      oauth2.AuthStyleInParams,
			},
		},
		{
			name: "invalid credentials",
			config: clientcredentials.Config{
				ClientID:     "wrong-id",
				ClientSecret: "wrong-secret",
				TokenURL:     server.URL + "/token",
				AuthStyle:    oauth2.AuthStyleInParams,
			},
			wantErr: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			src := &oAuthTokenSource{
				source: tc.config.TokenSource(context.Background()),
			}

			token, err := src.Token(context.Background())

			if tc.wantErr {
				assert.Error(t, err)
				return
			}

			require.NoError(t, err)
			assert.True(t, strings.HasPrefix("Bearer "+token, "Bearer "),
				fmt.Sprintf("expected non-empty token, got: %s", token))
			assert.NotEmpty(t, token)
		})
	}
}
