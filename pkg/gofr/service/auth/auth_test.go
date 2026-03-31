package auth

import (
	"context"
	"encoding/base64"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/oauth2"

	"gofr.dev/pkg/gofr/logging"
	"gofr.dev/pkg/gofr/service"
)

func TestAuthProvider(t *testing.T) {
	basicAuthServer := setupTestServer(t, func(r *http.Request) int {
		expected := "Basic " + base64.StdEncoding.EncodeToString([]byte("username:password"))
		if r.Header.Get(service.AuthHeader) == expected {
			return http.StatusOK
		}

		return http.StatusUnauthorized
	})

	apiKeyServer := setupTestServer(t, func(r *http.Request) int {
		if r.Header.Get("X-Api-Key") == "valid-key" {
			return http.StatusOK
		}

		return http.StatusUnauthorized
	})

	clientID, err := generateRandomString(clientIDLength)
	require.NoError(t, err)

	clientSecret, err := generateRandomString(clientSecretLength)
	require.NoError(t, err)

	oauthServer := setupOAuthHTTPServer(t, clientID, clientSecret, "test-aud")

	validBasicAuth, err := NewBasicAuthConfig("username", "cGFzc3dvcmQ=")
	require.NoError(t, err)

	invalidBasicAuth := NewAuthOption(&basicAuthConfig{userName: "username", password: "wrong", headerValue: "Basic wrong"})

	validAPIKey, err := NewAPIKeyConfig("valid-key")
	require.NoError(t, err)

	invalidAPIKey, err := NewAPIKeyConfig("invalid-key")
	require.NoError(t, err)

	validOAuth, err := NewOAuthConfig(clientID, clientSecret, oauthServer.URL+"/token",
		nil, url.Values{"aud": {"test-aud"}}, oauth2.AuthStyleInParams)
	require.NoError(t, err)

	testCases := []struct {
		name       string
		server     *httptest.Server
		authOption service.Options
		headers    map[string]string
		statusCode int
		wantErr    bool
	}{
		{name: "valid basic auth", server: basicAuthServer, authOption: validBasicAuth, statusCode: http.StatusOK},
		{name: "invalid basic auth", server: basicAuthServer, authOption: invalidBasicAuth, statusCode: http.StatusUnauthorized},
		{name: "valid api key", server: apiKeyServer, authOption: validAPIKey, statusCode: http.StatusOK},
		{name: "invalid api key", server: apiKeyServer, authOption: invalidAPIKey, statusCode: http.StatusUnauthorized},
		{name: "valid oauth", server: oauthServer, authOption: validOAuth, statusCode: http.StatusOK},
		{name: "existing auth header collision", server: basicAuthServer, authOption: validBasicAuth,
			headers: map[string]string{service.AuthHeader: "existing"}, wantErr: true},
	}

	httpMethods := []string{http.MethodGet, http.MethodPost, http.MethodPut, http.MethodPatch, http.MethodDelete}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			httpService := service.NewHTTPService(tc.server.URL, logging.NewMockLogger(logging.INFO), nil, tc.authOption)

			for _, method := range httpMethods {
				resp, err := callHTTPMethod(t.Context(), httpService, method, tc.headers)

				if tc.wantErr {
					require.Error(t, err)
					continue
				}

				if err != nil {
					continue
				}

				assert.Equal(t, tc.statusCode, resp.StatusCode)

				resp.Body.Close()
			}
		})
	}
}

func setupTestServer(t *testing.T, check func(r *http.Request) int) *httptest.Server {
	t.Helper()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(check(r))
	}))

	t.Cleanup(func() { server.Close() })

	return server
}

func callHTTPMethod(ctx context.Context, svc service.HTTP, method string,
	headers map[string]string) (resp *http.Response, err error) {
	path := "test"
	queryParams := map[string]any{"key": "value"}
	body := []byte("body")

	if headers != nil {
		return callWithHeaders(ctx, svc, method, path, queryParams, body, headers)
	}

	return callWithoutHeaders(ctx, svc, method, path, queryParams, body)
}

func callWithHeaders(ctx context.Context, svc service.HTTP, method, path string,
	queryParams map[string]any, body []byte, headers map[string]string) (*http.Response, error) {
	switch method {
	case http.MethodGet:
		return svc.GetWithHeaders(ctx, path, queryParams, headers)
	case http.MethodPost:
		return svc.PostWithHeaders(ctx, path, queryParams, body, headers)
	case http.MethodPut:
		return svc.PutWithHeaders(ctx, path, queryParams, body, headers)
	case http.MethodPatch:
		return svc.PatchWithHeaders(ctx, path, queryParams, body, headers)
	default:
		return svc.DeleteWithHeaders(ctx, path, body, headers)
	}
}

func callWithoutHeaders(ctx context.Context, svc service.HTTP, method, path string,
	queryParams map[string]any, body []byte) (*http.Response, error) {
	switch method {
	case http.MethodGet:
		return svc.Get(ctx, path, queryParams)
	case http.MethodPost:
		return svc.Post(ctx, path, queryParams, body)
	case http.MethodPut:
		return svc.Put(ctx, path, queryParams, body)
	case http.MethodPatch:
		return svc.Patch(ctx, path, queryParams, body)
	default:
		return svc.Delete(ctx, path, body)
	}
}
