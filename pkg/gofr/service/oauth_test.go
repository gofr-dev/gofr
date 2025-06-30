package service

import (
	"context"
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
	server := setupOAuthHTTPServer(t)

	tokenURL := server.getTokenURL()
	clientID := server.clientID
	clientSecret := server.clientSecret

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
	server := setupOAuthHTTPServer(t)

	tokenURL := server.getTokenURL()

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
			service, ok := getOAuthService(server.httpService(), server.clientID, server.clientSecret, tc.tokenURL, server.audienceClaim).(*oAuth)
			assert.True(t, ok, "unable to get oAuth object for test case #%d", i)

			headers, err := service.addAuthorizationHeader(t.Context(), tc.headers)
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

func TestHttpService_RequestsOAuth(t *testing.T) {
	server := setupOAuthHTTPServer(t)

	tokenURL := server.getTokenURL()

	testCases := []struct {
		method     string
		headers    bool
		tokenURL   string
		secret     string
		err        error
		statusCode int
	}{
		// Success
		{method: http.MethodGet, headers: true, tokenURL: tokenURL, secret: server.clientSecret, statusCode: http.StatusOK},
		{method: http.MethodPost, headers: true, tokenURL: tokenURL, secret: server.clientSecret, statusCode: http.StatusOK},
		{method: http.MethodPut, headers: true, tokenURL: tokenURL, secret: server.clientSecret, statusCode: http.StatusOK},
		{method: http.MethodDelete, headers: true, tokenURL: tokenURL, secret: server.clientSecret, statusCode: http.StatusOK},
		{method: http.MethodPatch, headers: true, tokenURL: tokenURL, secret: server.clientSecret, statusCode: http.StatusOK},
		{method: http.MethodGet, tokenURL: tokenURL, secret: server.clientSecret, statusCode: http.StatusOK},
		{method: http.MethodPost, tokenURL: tokenURL, secret: server.clientSecret, statusCode: http.StatusOK},
		{method: http.MethodPut, tokenURL: tokenURL, secret: server.clientSecret, statusCode: http.StatusOK},
		{method: http.MethodDelete, tokenURL: tokenURL, secret: server.clientSecret, statusCode: http.StatusOK},
		{method: http.MethodPatch, tokenURL: tokenURL, secret: server.clientSecret, statusCode: http.StatusOK},

		// Missing credentials
		{method: http.MethodGet, tokenURL: tokenURL, err: errInvalidCredentials, statusCode: http.StatusBadRequest},
		{method: http.MethodPost, tokenURL: tokenURL, err: errInvalidCredentials, statusCode: http.StatusBadRequest},
		{method: http.MethodPut, tokenURL: tokenURL, err: errInvalidCredentials, statusCode: http.StatusBadRequest},
		{method: http.MethodDelete, tokenURL: tokenURL, err: errInvalidCredentials, statusCode: http.StatusBadRequest},
		{method: http.MethodPatch, tokenURL: tokenURL, err: errInvalidCredentials, statusCode: http.StatusBadRequest},

		// Invalid credentials
		{method: http.MethodGet, tokenURL: tokenURL, secret: "lorem_ipsum", err: errInvalidCredentials, statusCode: http.StatusUnauthorized},
		{method: http.MethodPost, tokenURL: tokenURL, secret: "lorem_ipsum", err: errInvalidCredentials, statusCode: http.StatusUnauthorized},
		{method: http.MethodPut, tokenURL: tokenURL, secret: "lorem_ipsum", err: errInvalidCredentials, statusCode: http.StatusUnauthorized},
		{method: http.MethodDelete, tokenURL: tokenURL, secret: "lorem_ipsum", err: errInvalidCredentials, statusCode: http.StatusUnauthorized},
		{method: http.MethodPatch, tokenURL: tokenURL, secret: "lorem_ipsum", err: errInvalidCredentials, statusCode: http.StatusUnauthorized},

		// Missing Token URL
		{method: http.MethodGet, secret: server.clientSecret, err: errMissingTokenURL},
		{method: http.MethodPost, secret: server.clientSecret, err: errMissingTokenURL},
		{method: http.MethodPut, secret: server.clientSecret, err: errMissingTokenURL},
		{method: http.MethodDelete, secret: server.clientSecret, err: errMissingTokenURL},
		{method: http.MethodPatch, secret: server.clientSecret, err: errMissingTokenURL},

		// Invalid Token URL
		{method: http.MethodGet, tokenURL: invalidURL, secret: server.clientSecret, err: errIncorrectProtocol},
		{method: http.MethodPost, tokenURL: invalidURL, secret: server.clientSecret, err: errIncorrectProtocol},
		{method: http.MethodPut, tokenURL: invalidURL, secret: server.clientSecret, err: errIncorrectProtocol},
		{method: http.MethodDelete, tokenURL: invalidURL, secret: server.clientSecret, err: errIncorrectProtocol},
		{method: http.MethodPatch, tokenURL: invalidURL, secret: server.clientSecret, err: errIncorrectProtocol},
	}

	for i, tc := range testCases {
		t.Run(fmt.Sprintf("Test Case #%d", i), func(t *testing.T) {
			var resp *http.Response

			var err error

			service := server.httpService()
			service = getOAuthService(service, server.clientID, tc.secret, tc.tokenURL, server.audienceClaim)

			if tc.headers {
				resp, err = callHTTPServiceWithHeaders(t.Context(), service, tc.method)
			} else {
				resp, err = callHTTPServiceWithoutHeaders(t.Context(), service, tc.method)
			}

			retrieveError := &oauth2.RetrieveError{}
			URLError := &url.Error{}

			if errors.As(err, &retrieveError) {
				assert.Equal(t, tc.statusCode, retrieveError.Response.StatusCode)
			} else if errors.As(err, &URLError) {
				assert.Equal(t, tc.err, URLError.Err)
				assert.Equal(t, tc.tokenURL, URLError.URL)
			} else if err != nil {
				t.Errorf("Unknown error type encountered %v", err)
			}

			if resp != nil {
				assert.Equal(t, tc.statusCode, resp.StatusCode)

				if err = resp.Body.Close(); err != nil {
					t.Errorf("error in closing response %v", err)
				}
			}
		})
	}
}

func callHTTPServiceWithHeaders(ctx context.Context, service HTTP, method string) (resp *http.Response, err error) {
	path := "test"
	queryParams := map[string]any{"key": "value"}
	body := []byte("body")
	switch method {
	case http.MethodGet:
		return service.GetWithHeaders(ctx, path, queryParams, nil)
	case http.MethodPost:
		return service.PostWithHeaders(ctx, path, queryParams, body, nil)
	case http.MethodPut:
		return service.PutWithHeaders(ctx, path, queryParams, body, nil)
	case http.MethodPatch:
		return service.PatchWithHeaders(ctx, path, queryParams, body, nil)
	case http.MethodDelete:
		return service.DeleteWithHeaders(ctx, path, body, nil)
	default:
		return nil, nil
	}
}

func callHTTPServiceWithoutHeaders(ctx context.Context, service HTTP, method string) (resp *http.Response, err error) {
	path := "test"
	queryParams := map[string]any{"key": "value"}
	body := []byte("body")
	switch method {
	case http.MethodGet:
		return service.Get(ctx, path, queryParams)
	case http.MethodPost:
		return service.Post(ctx, path, queryParams, body)
	case http.MethodPut:
		return service.Put(ctx, path, queryParams, body)
	case http.MethodPatch:
		return service.Patch(ctx, path, queryParams, body)
	case http.MethodDelete:
		return service.Delete(ctx, path, body)
	default:
		return nil, nil
	}
}
