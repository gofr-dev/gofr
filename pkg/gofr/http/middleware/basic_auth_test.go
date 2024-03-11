package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestBasicAuthMiddleware(t *testing.T) {
	validationFunc := func(user, pass string) bool {
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
			authHeader:         "basic dXNlcjpwYXNzd29yZA==",
			authProvider:       BasicAuthProvider{Users: map[string]string{"user": "password"}},
			expectedStatusCode: http.StatusOK,
		},
		{
			name:               "Valid Authorization with validation Func",
			authHeader:         "basic YWJjOnBhc3MxMjM=",
			authProvider:       BasicAuthProvider{ValidateFunc: validationFunc},
			expectedStatusCode: http.StatusOK,
		},
		{
			name:               "false from validation Func",
			authHeader:         "basic dXNlcjpwYXNzd29yZA==",
			authProvider:       BasicAuthProvider{ValidateFunc: validationFunc},
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
			authHeader:         "basic invalidbase64encoding==",
			authProvider:       BasicAuthProvider{},
			expectedStatusCode: http.StatusUnauthorized,
		},
		{
			name:               "improper credentials format",
			authHeader:         "basic dXNlcis=",
			authProvider:       BasicAuthProvider{},
			expectedStatusCode: http.StatusUnauthorized,
		},
		{
			name:               "Unauthorized",
			authHeader:         "basic dXNlcjpwYXNzd29yZA==",
			authProvider:       BasicAuthProvider{},
			expectedStatusCode: http.StatusUnauthorized,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
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
