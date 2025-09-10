package middleware

import (
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestAuthMiddleware(t *testing.T) {
	errBody := `{"error":{"message":"invalid auth header in key 'Authorization'"}}`

	testCases := []struct {
		url            string
		success        bool
		statusCode     int
		expectedHeader any
		expectedBody   string
	}{
		{url: "/.well-known/health", success: true, statusCode: http.StatusOK, expectedBody: `OK`},
		{url: "/.well-known/health", statusCode: http.StatusOK, expectedBody: `OK`},
		{url: "/", success: true, statusCode: http.StatusOK, expectedHeader: "user-header-string", expectedBody: `OK`},
		{url: "/", success: false, statusCode: http.StatusUnauthorized, expectedBody: errBody},
	}
	for i, tc := range testCases {
		t.Run(strconv.Itoa(i), func(t *testing.T) {
			authProvider := &MockAuthProvider{
				success:    tc.success,
				method:     Username,
				authHeader: "user-header-string",
			}

			mockHandler := &MockHandler{t: t, authMethod: authProvider.method, authHeader: tc.expectedHeader}
			middleware := AuthMiddleware(authProvider)
			handler := middleware(mockHandler)
			req := httptest.NewRequest(http.MethodGet, tc.url, http.NoBody)
			rr := httptest.NewRecorder()
			handler.ServeHTTP(rr, req)

			assert.Equal(t, tc.statusCode, rr.Code)
			assert.Equal(t, tc.statusCode == http.StatusOK, mockHandler.handlerCalled)
			assert.Equal(t, tc.expectedBody, strings.TrimSuffix(rr.Body.String(), "\n"))

			if strings.HasPrefix(tc.url, "/.well-known") {
				return
			}

			assert.True(t, authProvider.extractAuthHeaderCalled)

			assert.Equal(t, tc.statusCode == http.StatusOK, authProvider.getAuthMethodCalled)
		})
	}
}

func Test_getAuthHeaderValue(t *testing.T) {
	testCases := []struct {
		header string
		key    string
		prefix string
		result string
		err    ErrorHTTP
	}{
		{err: ErrorMissingAuthHeader{}},
		{key: headerAuthorization, prefix: "Bearer", err: ErrorMissingAuthHeader{key: headerAuthorization}},
		{key: headerAuthorization, prefix: "", err: ErrorMissingAuthHeader{key: headerAuthorization}},
		{key: headerAuthorization, prefix: "", header: "some value", result: "some value"},
		{key: headerAuthorization, prefix: "Bearer", err: ErrorMissingAuthHeader{key: headerAuthorization}},
		{header: "Basic", key: headerAuthorization, prefix: "Bearer", err: ErrorInvalidAuthorizationHeaderFormat{
			key:        headerAuthorization,
			errMessage: "header should be `Bearer <value>`",
		}},
		{header: "Bearer", key: headerAuthorization, prefix: "Bearer", err: ErrorInvalidAuthorizationHeaderFormat{
			key:        headerAuthorization,
			errMessage: "header should be `Bearer <value>`",
		}},
		{header: "Bearer  ", key: headerAuthorization, prefix: "Bearer", err: ErrorInvalidAuthorizationHeaderFormat{
			key:        headerAuthorization,
			errMessage: "header should be `Bearer <value>`",
		}},
		{header: "Bearer some-value", key: headerAuthorization, prefix: "Bearer", result: "some-value"},
	}
	for i, tc := range testCases {
		t.Run(strconv.Itoa(i), func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/", http.NoBody)
			req.Header.Set("Authorization", tc.header)
			result, err := getAuthHeaderFromRequest(req, tc.key, tc.prefix)
			assert.Equal(t, tc.result, result)
			assert.Equal(t, tc.err, err)
		})
	}
}

type MockAuthProvider struct {
	success                 bool
	method                  AuthMethod
	authHeader              any
	extractAuthHeaderCalled bool
	getAuthMethodCalled     bool
}

func (p *MockAuthProvider) GetAuthMethod() AuthMethod {
	p.getAuthMethodCalled = true
	return p.method
}
func (p *MockAuthProvider) ExtractAuthHeader(_ *http.Request) (any, ErrorHTTP) {
	p.extractAuthHeaderCalled = true
	if p.success {
		return p.authHeader, nil
	}

	return nil, ErrorInvalidAuthorizationHeader{key: headerAuthorization}
}

type MockHandler struct {
	t             *testing.T
	handlerCalled bool
	authMethod    AuthMethod
	authHeader    any
}

func (h *MockHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	h.t.Helper()
	h.handlerCalled = true
	authHeader := r.Context().Value(h.authMethod)
	assert.Equal(h.t, h.authHeader, authHeader)
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte("OK"))
}
