package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"

	"gofr.dev/pkg/gofr/container"
)

func TestBasicAuthMiddleware(t *testing.T) {
	validationFunc := func(user, pass string) bool {
		if user == "abc" && pass == "pass123" {
			return true
		}

		return false
	}

	validationFuncWithDB := func(_ *container.Container, user, pass string) bool {
		if user == "abc" && pass == "pass123" {
			return true
		}

		return false
	}

	testCases := []struct {
		name               string
		authHeader         string
		authProvider       BasicAuthProvider
		expectedStatusCode int
	}{
		{
			name:               "Valid Authorization",
			authHeader:         "Basic dXNlcjpwYXNzd29yZA==",
			authProvider:       BasicAuthProvider{Users: map[string]string{"user": "password"}},
			expectedStatusCode: http.StatusOK,
		},
		{
			name:               "Valid Authorization with validation Func",
			authHeader:         "Basic YWJjOnBhc3MxMjM=",
			authProvider:       BasicAuthProvider{Users: map[string]string{"abc": "pass123"}, ValidateFunc: validationFunc},
			expectedStatusCode: http.StatusOK,
		},
		{
			name:               "false from validation Func",
			authHeader:         "Basic dXNlcjpwYXNzd29yZA==",
			authProvider:       BasicAuthProvider{ValidateFunc: validationFunc},
			expectedStatusCode: http.StatusUnauthorized,
		},
		{
			name:               "false from validation Func with DB",
			authHeader:         "Basic dXNlcjpwYXNzd29yZA==",
			authProvider:       BasicAuthProvider{ValidateFuncWithDatasources: validationFuncWithDB},
			expectedStatusCode: http.StatusUnauthorized,
		},
		{
			name:               "No Authorization Header",
			authHeader:         "",
			authProvider:       BasicAuthProvider{},
			expectedStatusCode: http.StatusUnauthorized,
		},
		{
			name:               "Invalid Authorization Header",
			authHeader:         "Bearer token",
			authProvider:       BasicAuthProvider{},
			expectedStatusCode: http.StatusUnauthorized,
		},
		{
			name:               "Invalid encoding",
			authHeader:         "Basic invalidbase64encoding==",
			authProvider:       BasicAuthProvider{},
			expectedStatusCode: http.StatusUnauthorized,
		},
		{
			name:               "improper credentials format",
			authHeader:         "Basic dXNlcis=",
			authProvider:       BasicAuthProvider{},
			expectedStatusCode: http.StatusUnauthorized,
		},
		{
			name:               "Unauthorized",
			authHeader:         "Basic dXNlcjpwYXNzd29yZA==",
			authProvider:       BasicAuthProvider{},
			expectedStatusCode: http.StatusUnauthorized,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			handler := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(http.StatusOK)
			})

			req := httptest.NewRequest(http.MethodGet, "/", http.NoBody)
			req.Header.Set("Authorization", tc.authHeader)

			rr := httptest.NewRecorder()
			authMiddleware := BasicAuthMiddleware(tc.authProvider)

			authMiddleware(handler).ServeHTTP(rr, req)

			assert.Equal(t, tc.expectedStatusCode, rr.Code)
		})
	}
}

func Test_BasicAuthMiddleware_well_known(t *testing.T) {
	testHandler := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("Success"))
	})

	req := httptest.NewRequest(http.MethodGet, "/.well-known/health-check", http.NoBody)
	rr := httptest.NewRecorder()

	authMiddleware := BasicAuthMiddleware(BasicAuthProvider{})(testHandler)
	authMiddleware.ServeHTTP(rr, req)

	assert.Equal(t, 200, rr.Code, "TEST Failed.\n")

	assert.Equal(t, "Success", rr.Body.String(), "TEST Failed.\n")
}
