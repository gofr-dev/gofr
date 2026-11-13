package service

import (
	"context"
	"errors"
	"fmt"
	"maps"
	"net/http"
	"net/http/httptest"
	"net/url"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
	"golang.org/x/oauth2"

	"gofr.dev/pkg/gofr/logging"
)

func TestAuthProvider(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	validBasicAuthConfig := &BasicAuthConfig{UserName: "username", Password: "password"}
	basicAuthServer := setupBasicAuthHTTPServer(t, validBasicAuthConfig)
	invalidBasicAuthConfig := &BasicAuthConfig{UserName: "username", Password: "wrong-password"}

	validOAuthConfig := oAuthConfigForTests(t, "")
	oAuthServer := setupOAuthHTTPServer(t, validOAuthConfig)
	validOAuthConfig.TokenURL = oAuthServer.URL + "/token"

	invalidOAuthConfig := oAuthConfigForTests(t, oAuthServer.URL+"/token")
	invalidOAuthConfig2 := oAuthConfigForTests(t, "")
	invalidOAuthConfig3 := oAuthConfigForTests(t, invalidURL)

	validAPIKeyConfig := &APIKeyConfig{"valid-value"}
	apiKeyAuthServer := setupAPIKeyAuthHTTPServer(t, validAPIKeyConfig)
	invalidAPIKeyConfig := &APIKeyConfig{"invalid-value"}

	authHeaderExistsErr := AuthErr{Message: "a value already exists for header " + AuthHeader}
	apiHeaderExistsErr := AuthErr{Message: "a value already exists for header " + xAPIKeyHeader}

	testCases := []struct {
		authOption Options
		headers    map[string]string
		statusCode int
		err        error
	}{
		{authOption: validBasicAuthConfig, statusCode: http.StatusOK},
		{authOption: invalidBasicAuthConfig, statusCode: http.StatusUnauthorized},
		{authOption: validOAuthConfig, headers: map[string]string{AuthHeader: "auth-string"}, err: authHeaderExistsErr},

		{authOption: validAPIKeyConfig, statusCode: http.StatusOK},
		{authOption: invalidAPIKeyConfig, statusCode: http.StatusUnauthorized},
		{authOption: validAPIKeyConfig, headers: map[string]string{xAPIKeyHeader: "auth-string"}, err: apiHeaderExistsErr},

		{authOption: validOAuthConfig, statusCode: http.StatusOK},
		{authOption: invalidOAuthConfig, statusCode: http.StatusUnauthorized, err: errInvalidCredentials},
		{authOption: invalidOAuthConfig2, statusCode: http.StatusUnauthorized, err: errMissingTokenURL},
		{authOption: invalidOAuthConfig3, statusCode: http.StatusUnauthorized, err: errIncorrectProtocol},
		{authOption: validOAuthConfig, headers: map[string]string{AuthHeader: "auth-string"}, err: authHeaderExistsErr},
	}

	httpMethods := []string{http.MethodGet, http.MethodPost, http.MethodPut, http.MethodPatch, http.MethodDelete, methodQuery}

	for i, tc := range testCases {
		t.Run(fmt.Sprintf("Test Case #%d", i), func(t *testing.T) {
			var server *httptest.Server

			switch tc.authOption.(type) {
			case *OAuthConfig:
				server = oAuthServer
			case *BasicAuthConfig:
				server = basicAuthServer
			case *APIKeyConfig:
				server = apiKeyAuthServer
			}

			httpService := NewHTTPService(server.URL, logging.NewMockLogger(logging.INFO), nil, tc.authOption)

			for _, method := range httpMethods {
				resp, err := callHTTPService(t.Context(), httpService, method, tc.headers)

				validateOAuthError(t, err, tc.err, tc.statusCode)

				if err != nil {
					return
				}

				assert.Equal(t, tc.statusCode, resp.StatusCode)

				err = resp.Body.Close()
				if err != nil {
					t.Errorf("error closing response body %v", err)
				}
			}
		})
	}
}

func validateOAuthError(t *testing.T, err, expectedError error, statusCode int) {
	t.Helper()

	retrieveError := &oauth2.RetrieveError{}
	URLError := &url.Error{}
	authErr := AuthErr{}

	if errors.As(err, &retrieveError) {
		assert.Equal(t, statusCode, retrieveError.Response.StatusCode)
	} else if errors.As(err, &URLError) {
		assert.Equal(t, expectedError, URLError.Err)
	} else if errors.As(err, &authErr) {
		assert.Equal(t, expectedError, err)
	} else if err != nil {
		t.Errorf("Unknown error type encountered %v", err)
	}
}

func callHTTPService(ctx context.Context, service HTTP, method string,
	headers map[string]string) (resp *http.Response, err error) {
	if headers != nil {
		resp, err = callHTTPServiceWithHeaders(ctx, service, method, headers)
	} else {
		resp, err = callHTTPServiceWithoutHeaders(ctx, service, method)
	}

	return resp, err
}

func callHTTPServiceWithHeaders(ctx context.Context, service HTTP, method string,
	headers map[string]string) (resp *http.Response, err error) {
	path := "test"
	queryParams := map[string]any{"key": "value"}
	body := []byte("body")

	switch method {
	case http.MethodGet:
		return service.GetWithHeaders(ctx, path, queryParams, headers)
	case http.MethodPost:
		return service.PostWithHeaders(ctx, path, queryParams, body, headers)
	case http.MethodPut:
		return service.PutWithHeaders(ctx, path, queryParams, body, headers)
	case http.MethodPatch:
		return service.PatchWithHeaders(ctx, path, queryParams, body, headers)
	case http.MethodDelete:
		return service.DeleteWithHeaders(ctx, path, body, headers)
	case methodQuery:
		return service.QueryWithHeaders(ctx, path, queryParams, body, headers)
	default:
		return nil, AuthErr{Message: "unknown method"}
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
	case methodQuery:
		return service.Query(ctx, path, queryParams, body)
	default:
		return nil, AuthErr{Message: "unknown method"}
	}
}

var errAuthFailed = errors.New("auth failed")

func TestAuthProvider_AuthError(t *testing.T) {
	provider := &authProvider{
		auth: func(context.Context, map[string]string) (map[string]string, error) {
			return nil, errAuthFailed
		},
		HTTP: &mockHTTP{},
	}

	testCases := []struct {
		desc    string
		method  string
		headers map[string]string
		expErr  error
	}{
		{desc: "GET", method: http.MethodGet, expErr: errAuthFailed},
		{desc: "POST", method: http.MethodPost, expErr: errAuthFailed},
		{desc: "PUT", method: http.MethodPut, expErr: errAuthFailed},
		{desc: "PATCH", method: http.MethodPatch, expErr: errAuthFailed},
		{desc: "DELETE", method: http.MethodDelete, expErr: errAuthFailed},
		{desc: "QUERY", method: methodQuery, expErr: errAuthFailed},
		{desc: "GET with headers", method: http.MethodGet, headers: map[string]string{"k": "v"}, expErr: errAuthFailed},
		{desc: "POST with headers", method: http.MethodPost, headers: map[string]string{"k": "v"}, expErr: errAuthFailed},
		{desc: "PUT with headers", method: http.MethodPut, headers: map[string]string{"k": "v"}, expErr: errAuthFailed},
		{desc: "PATCH with headers", method: http.MethodPatch, headers: map[string]string{"k": "v"}, expErr: errAuthFailed},
		{desc: "DELETE with headers", method: http.MethodDelete, headers: map[string]string{"k": "v"}, expErr: errAuthFailed},
		{desc: "QUERY with headers", method: methodQuery, headers: map[string]string{"k": "v"}, expErr: errAuthFailed},
	}

	for _, tc := range testCases {
		t.Run(tc.desc, func(t *testing.T) {
			gotResponse, err := callHTTPServiceAndClose(t.Context(), provider, tc.method, tc.headers)

			require.ErrorIs(t, err, tc.expErr)
			assert.False(t, gotResponse)
		})
	}
}

// callHTTPServiceAndClose calls the service, closes any returned body and reports whether a response was returned.
func callHTTPServiceAndClose(ctx context.Context, service HTTP, method string,
	headers map[string]string) (gotResponse bool, err error) {
	resp, err := callHTTPService(ctx, service, method, headers)
	if resp == nil {
		return false, err
	}

	_ = resp.Body.Close()

	return true, err
}

// failOnceServer answers its first request with 502 Bad Gateway and every later one with 200, counting the
// requests that reached it. A request missing requiredHeader is answered with 401 instead.
func failOnceServer(t *testing.T, requiredHeader string) (*httptest.Server, *atomic.Int32) {
	t.Helper()

	var hits atomic.Int32

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if hits.Add(1) == 1 {
			w.WriteHeader(http.StatusBadGateway)
			return
		}

		if r.Header.Get(requiredHeader) == "" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}

		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(server.Close)

	return server, &hits
}

func TestHeaderOptions_LeaveCallerHeadersUntouched(t *testing.T) {
	oAuthConfig := oAuthConfigForTests(t, "")
	oAuthConfig.TokenURL = setupOAuthHTTPServer(t, oAuthConfig).URL + "/token"

	fileTokenConfig, err := NewFileTokenAuthConfig(WithTokenFilePath(writeTokenFile(t, "file-token")))
	require.NoError(t, err)
	t.Cleanup(func() { _ = fileTokenConfig.Close() })

	defaultHeaders := &DefaultHeaders{Headers: map[string]string{"X-Default": "d1"}}

	testCases := []struct {
		desc           string
		options        []Options
		requiredHeader string
		headers        map[string]string
	}{
		{desc: "basic auth inside retry", options: []Options{&BasicAuthConfig{UserName: "alice", Password: "secret"}},
			requiredHeader: AuthHeader, headers: map[string]string{"X-Tenant": "t1"}},
		{desc: "API key inside retry", options: []Options{&APIKeyConfig{APIKey: "sk-live-123"}},
			requiredHeader: xAPIKeyHeader, headers: map[string]string{"X-Tenant": "t1"}},
		{desc: "OAuth inside retry", options: []Options{oAuthConfig},
			requiredHeader: AuthHeader, headers: map[string]string{"X-Tenant": "t1"}},
		{desc: "file token inside retry", options: []Options{fileTokenConfig},
			requiredHeader: AuthHeader, headers: map[string]string{"X-Tenant": "t1"}},
		{desc: "default headers inside retry", options: []Options{defaultHeaders},
			requiredHeader: "X-Default", headers: map[string]string{"X-Tenant": "t1"}},
		{desc: "default headers outside retry, basic auth inside, nil headers",
			options:        []Options{&BasicAuthConfig{UserName: "alice", Password: "secret"}, &RetryConfig{MaxRetries: 3}, defaultHeaders},
			requiredHeader: AuthHeader},
	}

	for _, tc := range testCases {
		t.Run(tc.desc, func(t *testing.T) {
			server, hits := failOnceServer(t, tc.requiredHeader)

			options := tc.options
			if len(options) == 1 {
				options = append(options, &RetryConfig{MaxRetries: 3})
			}

			svc := NewHTTPService(server.URL, logging.NewMockLogger(logging.ERROR), nil, options...)

			before := maps.Clone(tc.headers)

			// The first call fails once and is retried; the second reuses the same headers map.
			for call := range 2 {
				resp, err := callHTTPService(t.Context(), svc, http.MethodGet, tc.headers)
				require.NoError(t, err, "call %d", call)
				assert.Equal(t, http.StatusOK, resp.StatusCode, "call %d", call)
				require.NoError(t, resp.Body.Close())
			}

			assert.Equal(t, int32(3), hits.Load(), "502, its retry, then the second call")
			assert.Equal(t, before, tc.headers, "the caller's headers map must not be modified")
		})
	}
}

func TestDefaultHeaders_SharedHeadersMapIsSafeAcrossGoroutines(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(server.Close)

	svc := NewHTTPService(server.URL, logging.NewMockLogger(logging.ERROR), nil,
		&DefaultHeaders{Headers: map[string]string{"X-Default": "d1", "X-Other": "o1"}})

	shared := map[string]string{"X-Tenant": "t1"}

	var wg sync.WaitGroup

	for range 50 {
		wg.Go(func() {
			resp, err := svc.GetWithHeaders(t.Context(), "test", nil, shared)
			if assert.NoError(t, err) {
				_ = resp.Body.Close()
			}
		})
	}

	wg.Wait()

	assert.Equal(t, map[string]string{"X-Tenant": "t1"}, shared)
}
