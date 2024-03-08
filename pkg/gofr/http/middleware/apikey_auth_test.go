package middleware

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
)

type mockAuthProvider struct {
	validateKeyFunc func(apiKey string) bool
}

func (m *mockAuthProvider) ValidateKey(apiKey string) bool {
	return m.validateKeyFunc(apiKey)
}

func Test_ApiKeyAuthMiddleware(t *testing.T) {
	authProvider := &mockAuthProvider{
		validateKeyFunc: func(apiKey string) bool {
			return apiKey == "valid-key"
		},
	}

	testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("Success"))
	})

	wrappedHandler := APIKeyAuthMiddleware(authProvider)(testHandler)

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
