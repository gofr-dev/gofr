package service

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"go.opentelemetry.io/otel"

	"github.com/golang-jwt/jwt/v5"
	"github.com/stretchr/testify/assert"

	"gofr.dev/pkg/gofr/testutil"
)

func oAuthHttpServer(t *testing.T) *httptest.Server {
	// Start a test HTTP server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		header := r.Header.Get("Authorization")
		token := strings.Split(header, " ")

		parsedToken, _ := jwt.Parse(token[1], func(token *jwt.Token) (interface{}, error) {
			return []byte("my-secret-key"), nil
		})

		claims, _ := parsedToken.Claims.GetAudience()

		assert.Equal(t, claims[0], "gofr-test")

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
		Logger: testutil.NewMockLogger(testutil.DEBUGLOG),
	}

	// Circuit breaker configuration
	oauthConfig := OAuthConfig{
		SigningMethod: jwt.SigningMethodHS256,
		Claims:        jwt.MapClaims{"aud": "gofr-test"},
		SecretKey:     `my-secret-key`,
		Validity:      10 * time.Hour,
	}

	// Apply circuit breaker option to the HTTP service
	httpSvc := oauthConfig.addOption(&service)

	return httpSvc
}

func setupHTTPServiceTestServerForOAuthWithUnSupportedMethod() HTTP {
	// Initialize HTTP service with custom transport, URL, tracer, logger, and metrics
	service := httpService{
		Client: &http.Client{},
		Tracer: otel.Tracer("gofr-http-client"),
		Logger: testutil.NewMockLogger(testutil.DEBUGLOG),
	}

	// Circuit breaker configuration
	oauthConfig := OAuthConfig{
		SigningMethod: jwt.SigningMethodRS256,
		Claims:        jwt.MapClaims{"aud": "gofr-test"},
		SecretKey:     `my-secret-key`,
		Validity:      10 * time.Hour,
	}

	// Apply circuit breaker option to the HTTP service
	httpSvc := oauthConfig.addOption(&service)

	return httpSvc
}

func TestHttpService_GetSuccessRequestsOAuth(t *testing.T) {
	server := oAuthHttpServer(t)

	service := setupHTTPServiceTestServerForOAuth(server)

	resp, err := service.Get(context.Background(), "test", nil)

	assert.Equal(t, resp.StatusCode, http.StatusOK)
	assert.Nil(t, err)

	_ = resp.Body.Close()
}

func TestHttpService_PostSuccessRequestsOAuth(t *testing.T) {
	server := oAuthHttpServer(t)

	service := setupHTTPServiceTestServerForOAuth(server)

	resp, err := service.Post(context.Background(), "test", nil, nil)

	assert.Equal(t, resp.StatusCode, http.StatusOK)
	assert.Nil(t, err)

	_ = resp.Body.Close()
}

func TestHttpService_PatchSuccessRequestsOAuth(t *testing.T) {
	server := oAuthHttpServer(t)

	service := setupHTTPServiceTestServerForOAuth(server)

	resp, err := service.Patch(context.Background(), "test", nil, nil)

	assert.Equal(t, resp.StatusCode, http.StatusOK)
	assert.Nil(t, err)

	_ = resp.Body.Close()
}

func TestHttpService_PutSuccessRequestsOAuth(t *testing.T) {
	server := oAuthHttpServer(t)

	service := setupHTTPServiceTestServerForOAuth(server)

	resp, err := service.Put(context.Background(), "test", nil, nil)

	assert.Equal(t, resp.StatusCode, http.StatusOK)
	assert.Nil(t, err)

	_ = resp.Body.Close()
}

func TestHttpService_DeleteSuccessRequestsOAuth(t *testing.T) {
	server := oAuthHttpServer(t)

	service := setupHTTPServiceTestServerForOAuth(server)

	resp, err := service.Delete(context.Background(), "test", nil)

	assert.Equal(t, resp.StatusCode, http.StatusOK)
	assert.Nil(t, err)

	_ = resp.Body.Close()
}

func TestHttpService_DeleteSuccessRequestsOAuthError(t *testing.T) {
	service := setupHTTPServiceTestServerForOAuthWithUnSupportedMethod()

	resp, err := service.Delete(context.Background(), "test", nil)

	assert.Nil(t, resp)
	assert.Contains(t, err.Error(), `key is of invalid type: RSA sign expects *rsa.PrivateKey`)
}

func TestHttpService_PutSuccessRequestsOAuthError(t *testing.T) {
	service := setupHTTPServiceTestServerForOAuthWithUnSupportedMethod()

	resp, err := service.Put(context.Background(), "test", nil, nil)

	assert.Nil(t, resp)
	assert.Contains(t, err.Error(), `key is of invalid type: RSA sign expects *rsa.PrivateKey`)
}

func TestHttpService_PatchSuccessRequestsOAuthError(t *testing.T) {
	service := setupHTTPServiceTestServerForOAuthWithUnSupportedMethod()

	resp, err := service.Patch(context.Background(), "test", nil, nil)

	assert.Nil(t, resp)
	assert.Contains(t, err.Error(), `key is of invalid type: RSA sign expects *rsa.PrivateKey`)
}

func TestHttpService_PostSuccessRequestsOAuthError(t *testing.T) {
	service := setupHTTPServiceTestServerForOAuthWithUnSupportedMethod()

	resp, err := service.Post(context.Background(), "test", nil, nil)

	assert.Nil(t, resp)
	assert.Contains(t, err.Error(), `key is of invalid type: RSA sign expects *rsa.PrivateKey`)
}

func TestHttpService_GetSuccessRequestsOAuthError(t *testing.T) {
	service := setupHTTPServiceTestServerForOAuthWithUnSupportedMethod()

	resp, err := service.Get(context.Background(), "test", nil)

	assert.Nil(t, resp)
	assert.Contains(t, err.Error(), `key is of invalid type: RSA sign expects *rsa.PrivateKey`)
}
