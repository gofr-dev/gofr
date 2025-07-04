package service

import (
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"golang.org/x/oauth2"
)

const invalidURL = "abc://invalid-url"

var (
	errMissingTokenURL    = errors.New(`unsupported protocol scheme ""`)
	errIncorrectProtocol  = errors.New(`unsupported protocol scheme "abc"`)
	errInvalidCredentials = &oauth2.RetrieveError{Response: &http.Response{StatusCode: http.StatusUnauthorized}}
)

func TestNewOAuthConfig(t *testing.T) {
	config := oAuthConfigForTests(t, "/token")

	server := setupOAuthHTTPServer(t, config)

	tokenURL := server.URL + config.TokenURL
	clientID := config.ClientID
	clientSecret := config.ClientSecret

	testCases := []struct {
		clientID     string
		clientSecret string
		tokenURL     string
		scopes       []string
		params       url.Values
		authStyle    oauth2.AuthStyle
		err          error
	}{
		{err: AuthErr{nil, "client id is mandatory"}},
		{clientID: clientID, err: AuthErr{nil, "client secret is mandatory"}},
		{clientID: clientID, tokenURL: tokenURL, err: AuthErr{nil, "client secret is mandatory"}},
		{clientID: clientID, clientSecret: clientSecret, err: AuthErr{nil, "token url is mandatory"}},
		{clientID: clientID, clientSecret: clientSecret, tokenURL: "invalid_url_format", err: AuthErr{nil, "empty host"}},
		{clientID: clientID, clientSecret: clientSecret, tokenURL: tokenURL},
		{clientID: clientID, clientSecret: "some_random_client_secret", tokenURL: tokenURL},
		{clientID: "some_random_client_id", clientSecret: clientSecret, tokenURL: tokenURL},
		{clientID: clientID, clientSecret: clientSecret, tokenURL: tokenURL, authStyle: 1},
		{clientID: clientID, clientSecret: "some_random_client_secret", tokenURL: tokenURL, authStyle: 1},
		{clientID: "some_random_client_id", clientSecret: clientSecret, tokenURL: tokenURL, authStyle: 2},
	}

	for i, tc := range testCases {
		t.Run(fmt.Sprintf("Test case #%d", i), func(t *testing.T) {
			config, err := NewOAuthConfig(tc.clientID, tc.clientSecret, tc.tokenURL, tc.scopes, tc.params, tc.authStyle)
			assert.Equal(t, tc.err, err)

			if tc.err != nil {
				assert.Empty(t, config)
				return
			}

			oAuthConfig, ok := config.(*OAuthConfig)
			assert.True(t, ok, "failed to get OAuthConfig")

			if oAuthConfig == nil {
				t.Errorf("failed to get OAuthConfig")
				return
			}

			assert.Equal(t, tc.clientID, oAuthConfig.ClientID)
			assert.Equal(t, tc.clientSecret, oAuthConfig.ClientSecret)
			assert.Equal(t, tc.tokenURL, oAuthConfig.TokenURL)
			assert.Equal(t, tc.params, oAuthConfig.EndpointParams)
			assert.Equal(t, tc.scopes, oAuthConfig.Scopes)
			assert.Equal(t, tc.authStyle, oAuthConfig.AuthStyle)
		})
	}
}

func TestHttpService_validateTokenURL(t *testing.T) {
	testCases := []struct {
		tokenURL string
		errMsg   string
	}{
		{tokenURL: "https://www.example.com"},
		{tokenURL: "https://www.example.com.", errMsg: "invalid host pattern, ends with `.`"},
		{tokenURL: "https://www.192.168.1.1.com"},
		{tokenURL: "https://www.192.168.1.1..com", errMsg: "invalid host pattern, contains `..`"},
		{tokenURL: "ftp://www.192.168.1.1..com", errMsg: "invalid host pattern, contains `..`"},
		{tokenURL: "ftp://www.192.168.1.1.com", errMsg: "invalid scheme, allowed http and https only"},
		{tokenURL: "www.192.168.1.1.com", errMsg: "empty host"},
		{tokenURL: "https://www.example.", errMsg: "invalid host pattern, ends with `.`"},
		{errMsg: "token url is mandatory"},
		{tokenURL: "invalid_url_format", errMsg: "empty host"},
	}

	for i, tc := range testCases {
		t.Run(fmt.Sprintf("Test Case #%d", i), func(t *testing.T) {
			err := validateTokenURL(tc.tokenURL)
			if tc.errMsg != "" {
				assert.ErrorContains(t, err, tc.errMsg)
			}
		})
	}
}

func TestAddAuthorizationHeader_OAuth(t *testing.T) {
	config := oAuthConfigForTests(t, "/token")

	server := setupOAuthHTTPServer(t, config)

	tokenURL := server.URL + config.TokenURL

	emptyHeaders := map[string]string{}
	headerWithAuth := map[string]string{AuthHeader: "Value"}
	headerWithEmptyAuth := map[string]string{AuthHeader: ""}
	headerWithoutAuth := map[string]string{"Content Type": "Value"}
	headerWithEmptyAuthAndOtherValues := map[string]string{"Content Type": "Value", AuthHeader: ""}
	authHeaderExistsError := AuthErr{Message: "value Value already exists for header Authorization"}

	testCases := []struct {
		tokenURL string
		headers  map[string]string
		response map[string]string
		err      error
	}{
		{headers: headerWithAuth, err: authHeaderExistsError},
		{err: &url.Error{Op: "Post", URL: "", Err: errMissingTokenURL}},
		{tokenURL: tokenURL, headers: headerWithAuth, err: authHeaderExistsError},
		{tokenURL: tokenURL, headers: headerWithEmptyAuth, response: emptyHeaders},
		{tokenURL: tokenURL, headers: headerWithoutAuth, response: headerWithoutAuth},
		{tokenURL: tokenURL, headers: headerWithEmptyAuthAndOtherValues, response: headerWithoutAuth},
	}

	for i, tc := range testCases {
		t.Run(fmt.Sprintf("Test Case #%d", i), func(t *testing.T) {
			config.TokenURL = tc.tokenURL

			headers, err := config.addAuthorizationHeader(t.Context(), tc.headers)
			assert.Equal(t, tc.err, err)

			if err != nil {
				return
			}

			authHeader, ok := headers[AuthHeader]
			assert.True(t, ok)
			assert.NotEmpty(t, authHeader)
			assert.True(t, strings.HasPrefix(authHeader, "Bearer"))
			delete(headers, AuthHeader)
			assert.Equal(t, tc.response, headers)
		})
	}
}

// Helper method for getting OAuthConfig.
func oAuthConfigForTests(t *testing.T, tokenURL string) *OAuthConfig {
	t.Helper()

	config := &OAuthConfig{
		TokenURL: tokenURL,
		EndpointParams: map[string][]string{
			"aud": {"some-random-value"},
		},
		AuthStyle: oauth2.AuthStyleInParams,
	}

	clientID, err := generateRandomString(clientIDLength)
	if err != nil {
		t.Fatalf("unable to generate random string for oAuthConfig")
	}

	config.ClientID = clientID

	clientSecret, err := generateRandomString(clientSecretLength)
	if err != nil {
		t.Fatalf("unable to generate random string for oAuthConfig")
	}

	config.ClientSecret = clientSecret

	return config
}
