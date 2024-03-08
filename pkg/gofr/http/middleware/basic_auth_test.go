package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
)

type MockAuthProvider struct {
	Username string
	Password string
	Result   bool
}

func (m *MockAuthProvider) ValidateUser(username, password string) bool {
	return m.Username == username && m.Password == password && m.Result
}

func TestBasicAuthMiddleware(t *testing.T) {
	testCases := []struct {
		name               string
		authHeader         string
		authProvider       *MockAuthProvider
		expectedStatusCode int
	}{
		{
			name:               "Valid Authorization",
			authHeader:         "basic dXNlcjpwYXNzd29yZA==",
			authProvider:       &MockAuthProvider{Username: "user", Password: "password", Result: true},
			expectedStatusCode: http.StatusOK,
		},
		{
			name:               "No Authorization Header",
			authHeader:         "",
			authProvider:       &MockAuthProvider{Username: "user", Password: "password", Result: true},
			expectedStatusCode: http.StatusUnauthorized,
		},
		{
			name:               "Invalid Authorization Header",
			authHeader:         "Bearer token",
			authProvider:       &MockAuthProvider{Username: "user", Password: "password", Result: true},
			expectedStatusCode: http.StatusUnauthorized,
		},
		{
			name:               "Invalid encoding",
			authHeader:         "basic invalidbase64encoding==",
			authProvider:       &MockAuthProvider{Username: "user", Password: "password", Result: true},
			expectedStatusCode: http.StatusUnauthorized,
		},
		{
			name:               "Unauthorized",
			authHeader:         "basic dXNlcjpwYXNzd29yZA==",
			authProvider:       &MockAuthProvider{Username: "user", Password: "wrongpassword", Result: false},
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
