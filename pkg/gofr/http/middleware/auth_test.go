package middleware

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var errSimulatedWriteFailure = errors.New("simulated write failure")

func TestAuthMiddleware(t *testing.T) {
	testCases := []struct {
		url            string
		success        bool
		statusCode     int
		expectedHeader any
	}{
		{url: "/.well-known/health", success: true, statusCode: http.StatusOK},
		{url: "/.well-known/health", statusCode: http.StatusOK},
		{url: "/", success: true, statusCode: http.StatusOK, expectedHeader: "user-header-string"},
		{url: "/", success: false, statusCode: http.StatusUnauthorized},
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

// Test for writeJSONError function which should write a JSON error response
func Test_writeJSONError(t *testing.T) {
	rr := httptest.NewRecorder()
	message := "Test error message"
	statusCode := http.StatusBadRequest

	writeJSONError(rr, message, statusCode)

	assert.Equal(t, statusCode, rr.Code)
	assert.Equal(t, "application/json", rr.Header().Get("Content-Type"))

	// Verify JSON structure matches our ErrorDetail/ErrMiddlewareResp structure
	var response ErrMiddlewareResp
	err := json.Unmarshal(rr.Body.Bytes(), &response)
	require.NoError(t, err)
	assert.Equal(t, message, response.Error.Message)
}

// Test for the error fallback when JSON encoding fails
func Test_writeJSONError_EncodingFailure(t *testing.T) {
	mockWriter := &mockFailingResponseWriter{
		header: http.Header{},
	}

	message := "Test error message"
	statusCode := http.StatusBadRequest

	writeJSONError(mockWriter, message, statusCode)

	assert.Equal(t, statusCode, mockWriter.statusCode)
	assert.Equal(t, message+"\n", mockWriter.body)
}

// Simplified mock response writer.
type mockFailingResponseWriter struct {
	header     http.Header
	body       string
	statusCode int
	jsonFailed bool // Track if JSON encoding has been attempted
}

func (m *mockFailingResponseWriter) Header() http.Header {
	return m.header
}

func (m *mockFailingResponseWriter) Write(b []byte) (int, error) {
	if !m.jsonFailed {
		m.jsonFailed = true
		return 0, errSimulatedWriteFailure
	}

	// Second write attempt (plain text fallback) will succeed
	m.body = string(b)

	return len(b), nil
}

func (m *mockFailingResponseWriter) WriteHeader(statusCode int) {
	m.statusCode = statusCode
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

	return nil, ErrorInvalidAuthorizationHeader{}
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
