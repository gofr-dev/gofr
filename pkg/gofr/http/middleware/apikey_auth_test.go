package middleware

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
)

func Test_ApiKeyAuthMiddleware(t *testing.T) {
	testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("Success"))
	})

	wrappedHandler := APIKeyAuthMiddleware("valid-key-1", "valid-key-2")(testHandler)

	req, err := http.NewRequestWithContext(context.Background(), "GET", "/", http.NoBody)
	if err != nil {
		t.Fatal(err)
	}

	testCases := []struct {
		desc         string
		apiKey       string
		responseCode int
		responseBody string
	}{
		{"missing api-key", "", 401, "Unauthorized\n"},
		{"invalid api-key", "invalid-key", 401, "Unauthorized\n"},
		{"valid api-key", "valid-key-1", 200, "Success"},
		{"another valid api-key", "valid-key-2", 200, "Success"},
	}

	for i, tc := range testCases {
		req.Header.Set("X-API-KEY", tc.apiKey)

		rr := httptest.NewRecorder()

		wrappedHandler.ServeHTTP(rr, req)

		assert.Equal(t, tc.responseCode, rr.Code, "TEST[%d], Failed.\n%s", i, tc.desc)

		assert.Equal(t, tc.responseBody, rr.Body.String(), "TEST[%d], Failed.\n%s", i, tc.desc)
	}
}

func Test_ApiKeyAuthMiddlewareWithFunc(t *testing.T) {
	testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("Success"))
	})

	validator := func(apiKey string) bool {
		return apiKey == "valid-key"
	}

	wrappedHandler := APIKeyAuthMiddlewareWithFunc(validator)(testHandler)

	req, err := http.NewRequestWithContext(context.Background(), "GET", "/", http.NoBody)
	if err != nil {
		t.Fatal(err)
	}

	testCases := []struct {
		desc         string
		apiKey       string
		responseCode int
		responseBody string
	}{
		{"missing api-key", "", 401, "Unauthorized\n"},
		{"invalid api-key", "invalid-key", 401, "Unauthorized\n"},
		{"valid api-key", "valid-key", 200, "Success"},
	}

	for i, tc := range testCases {
		req.Header.Set("X-API-KEY", tc.apiKey)

		rr := httptest.NewRecorder()

		wrappedHandler.ServeHTTP(rr, req)

		assert.Equal(t, tc.responseCode, rr.Code, "TEST[%d], Failed.\n%s", i, tc.desc)

		assert.Equal(t, tc.responseBody, rr.Body.String(), "TEST[%d], Failed.\n%s", i, tc.desc)
	}
}
