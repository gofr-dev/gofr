package service

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/golang-jwt/jwt/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel"

	"gofr.dev/pkg/gofr/logging"
)

func oAuthHTTPServer(t *testing.T) *httptest.Server {
	t.Helper()

	// Start a test HTTP server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		header := r.Header.Get("Authorization")
		token := strings.Split(header, " ")

		parsedToken, _ := jwt.Parse(token[1], func(*jwt.Token) (interface{}, error) {
			return []byte("my-secret-key"), nil
		})

		claims, _ := parsedToken.Claims.GetAudience()

		assert.Equal(t, "https://dev-zq6tvaxf3v7p0g7j.us.auth0.com/api/v2/", claims[0])

		w.WriteHeader(http.StatusOK)
	}))

	return server
}

func setupHTTPServiceTestServerForOAuth(server *httptest.Server) HTTP {
	// Initialize HTTP service with custom transport, URL, tracer, logger, and metrics
	service := httpService{
		Client: &http.Client{},
		url:    server.URL,
		Tracer: otel.Tracer("gofr-http-client"),
		Logger: logging.NewMockLogger(logging.DEBUG),
	}

	// Circuit breaker configuration
	oauthConfig := OAuthConfig{
		ClientID:     "0iyeGcLYWudLGqZfD6HvOdZHZ5TlciAJ",
		ClientSecret: "GQXTY2f9186nUS3C9WWi7eJz8-iVEsxq7lKxdjfhOJbsEPPtEszL3AxFn8k_NAER",
		TokenURL:     "https://dev-zq6tvaxf3v7p0g7j.us.auth0.com/oauth/token",
		EndpointParams: map[string][]string{
			"audience": {"https://dev-zq6tvaxf3v7p0g7j.us.auth0.com/api/v2/"},
		},
	}

	// Apply circuit breaker option to the HTTP service
	httpSvc := oauthConfig.AddOption(&service)

	return httpSvc
}

func setupHTTPServiceTestServerForOAuthWithUnSupportedMethod() HTTP {
	// Initialize HTTP service with custom transport, URL, tracer, logger, and metrics
	service := httpService{
		Client: &http.Client{},
		Tracer: otel.Tracer("gofr-http-client"),
		Logger: logging.NewMockLogger(logging.DEBUG),
	}

	// Circuit breaker configuration
	oauthConfig := OAuthConfig{}

	// Apply circuit breaker option to the HTTP service
	httpSvc := oauthConfig.AddOption(&service)

	return httpSvc
}

func TestHttpService_GetSuccessRequestsOAuth(t *testing.T) {
	server := oAuthHTTPServer(t)

	service := setupHTTPServiceTestServerForOAuth(server)

	resp, err := service.Get(context.Background(), "test", nil)

	assert.Equal(t, http.StatusOK, resp.StatusCode)
	require.NoError(t, err)

	_ = resp.Body.Close()
}

func TestHttpService_PostSuccessRequestsOAuth(t *testing.T) {
	server := oAuthHTTPServer(t)

	service := setupHTTPServiceTestServerForOAuth(server)

	resp, err := service.Post(context.Background(), "test", nil, nil)

	assert.Equal(t, http.StatusOK, resp.StatusCode)
	require.NoError(t, err)

	_ = resp.Body.Close()
}

func TestHttpService_PatchSuccessRequestsOAuth(t *testing.T) {
	server := oAuthHTTPServer(t)

	service := setupHTTPServiceTestServerForOAuth(server)

	resp, err := service.Patch(context.Background(), "test", nil, nil)

	assert.Equal(t, http.StatusOK, resp.StatusCode)
	require.NoError(t, err)

	_ = resp.Body.Close()
}

func TestHttpService_PutSuccessRequestsOAuth(t *testing.T) {
	server := oAuthHTTPServer(t)

	service := setupHTTPServiceTestServerForOAuth(server)

	resp, err := service.Put(context.Background(), "test", nil, nil)

	assert.Equal(t, http.StatusOK, resp.StatusCode)
	require.NoError(t, err)

	_ = resp.Body.Close()
}

func TestHttpService_DeleteSuccessRequestsOAuth(t *testing.T) {
	server := oAuthHTTPServer(t)

	service := setupHTTPServiceTestServerForOAuth(server)

	resp, err := service.Delete(context.Background(), "test", nil)

	assert.Equal(t, http.StatusOK, resp.StatusCode)
	require.NoError(t, err)

	_ = resp.Body.Close()
}

func TestHttpService_DeleteRequestsOAuthError(t *testing.T) {
	service := setupHTTPServiceTestServerForOAuthWithUnSupportedMethod()

	resp, err := service.Delete(context.Background(), "test", nil)

	assert.Nil(t, resp)
	require.ErrorContains(t, err, `unsupported protocol scheme`)

	if resp != nil {
		resp.Body.Close()
	}
}

func TestHttpService_PutRequestsOAuthError(t *testing.T) {
	service := setupHTTPServiceTestServerForOAuthWithUnSupportedMethod()

	resp, err := service.Put(context.Background(), "test", nil, nil)

	assert.Nil(t, resp)
	require.ErrorContains(t, err, `unsupported protocol scheme`)

	if resp != nil {
		resp.Body.Close()
	}
}

func TestHttpService_PatchRequestsOAuthError(t *testing.T) {
	service := setupHTTPServiceTestServerForOAuthWithUnSupportedMethod()

	resp, err := service.Patch(context.Background(), "test", nil, nil)

	assert.Nil(t, resp)
	require.ErrorContains(t, err, `unsupported protocol scheme`)

	if resp != nil {
		resp.Body.Close()
	}
}

func TestHttpService_PostRequestsOAuthError(t *testing.T) {
	service := setupHTTPServiceTestServerForOAuthWithUnSupportedMethod()

	resp, err := service.Post(context.Background(), "test", nil, nil)

	assert.Nil(t, resp)
	require.ErrorContains(t, err, `unsupported protocol scheme`)

	if resp != nil {
		resp.Body.Close()
	}
}

func TestHttpService_GetRequestsOAuthError(t *testing.T) {
	service := setupHTTPServiceTestServerForOAuthWithUnSupportedMethod()

	resp, err := service.Get(context.Background(), "test", nil)

	assert.Nil(t, resp)
	require.ErrorContains(t, err, `unsupported protocol scheme`)

	if resp != nil {
		resp.Body.Close()
	}
}
