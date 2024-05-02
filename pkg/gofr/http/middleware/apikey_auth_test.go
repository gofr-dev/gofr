package middleware

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
)

func Test_ApiKeyAuthMiddleware(t *testing.T) {
	testHandler := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("Success"))
	})

	validator := func(apiKey string) bool {
		return apiKey == "valid-key"
	}

	req, err := http.NewRequestWithContext(context.Background(), "GET", "/", http.NoBody)
	if err != nil {
		t.Fatal(err)
	}

	testCases := []struct {
		desc         string
		validator    func(apiKey string) bool
		apiKey       string
		responseCode int
		responseBody string
	}{
		{"missing api-key", nil, "", 401, "Unauthorized: Authorization header missing\n"},
		{"invalid api-key", nil, "invalid-key", 401, "Unauthorized: Invalid Authorization header\n"},
		{"valid api-key", nil, "valid-key-1", 200, "Success"},
		{"another valid api-key", nil, "valid-key-2", 200, "Success"},
		{"custom validator valid key", validator, "valid-key", 200, "Success"},
		{"custom validator in-valid key", validator, "invalid-key", 401, "Unauthorized: Invalid Authorization header\n"},
	}

	for i, tc := range testCases {
		rr := httptest.NewRecorder()

		req.Header.Set("X-API-KEY", tc.apiKey)

		wrappedHandler := APIKeyAuthMiddleware(tc.validator, "valid-key-1", "valid-key-2")(testHandler)
		wrappedHandler.ServeHTTP(rr, req)

		assert.Equal(t, tc.responseCode, rr.Code, "TEST[%d], Failed.\n%s", i, tc.desc)

		assert.Equal(t, tc.responseBody, rr.Body.String(), "TEST[%d], Failed.\n%s", i, tc.desc)
	}
}

func Test_ApiKeyAuthMiddleware_well_known(t *testing.T) {
	testHandler := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("Success"))
	})

	req := httptest.NewRequest(http.MethodGet, "/.well-known/health-check", http.NoBody)
	rr := httptest.NewRecorder()

	wrappedHandler := APIKeyAuthMiddleware(nil)(testHandler)
	wrappedHandler.ServeHTTP(rr, req)

	assert.Equal(t, 200, rr.Code, "TEST Failed.\n")

	assert.Equal(t, "Success", rr.Body.String(), "TEST Failed.\n")
}
