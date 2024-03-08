package middleware

import (
	"github.com/stretchr/testify/assert"
	"net/http"
	"net/http/httptest"
	"testing"
)

func Test_OAuth(t *testing.T) {
	OAuth()
}

func TestOAuth_EmptyAuthorizationHeader(t *testing.T) {
	// Arrange
	// Create a mock inner handler
	innerHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("Inner handler called"))
	})

	// Create a mock request with an empty Authorization header
	req, _ := http.NewRequest("GET", "/", nil)
	req.Header.Set("Authorization", "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJzdWIiOiIxMjM0NTY3ODkwIiwibmFtZSI6IkpvaG4gRG9lIiwiaWF0IjoxNTE2MjM5MDIyfQ.SflKxwRJSMeKKF2QT4fwpMeJf36POk6yJV_adQssw5c")

	// Create a mock response recorder
	rr := httptest.NewRecorder()

	// Create the OAuth middleware
	middleware := OAuth()

	// Act
	// Call the OAuth middleware with the mock inner handler
	middleware(innerHandler).ServeHTTP(rr, req)

	// Assert
	// Check that the response status code is Unauthorized
	assert.Equal(t, http.StatusUnauthorized, rr.Code)
	// Check that the response body contains the error message
	assert.Equal(t, "Token is missing", rr.Body.String())
}
