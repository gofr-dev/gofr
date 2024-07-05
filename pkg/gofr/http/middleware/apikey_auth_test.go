package middleware

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"

	"gofr.dev/pkg/gofr/container"
)

const (
	validKey1 string = "valid-key-1"
	validKey2 string = "valid-key-2"
)

func Test_ApiKeyAuthMiddleware(t *testing.T) {
	testHandler := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("Success"))
	})

	validator := func(apiKey string) bool {
		return apiKey == validKey1
	}
	validatorWithDB := func(_ *container.Container, apiKey string) bool {
		return apiKey == validKey2
	}

	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, "/", http.NoBody)
	if err != nil {
		t.Fatal(err)
	}

	testCases := []struct {
		desc                string
		validatorFunc       func(akiKey string) bool
		validatorFuncWithDB func(c *container.Container, apiKey string) bool
		apiKey              string
		responseCode        int
		responseBody        string
	}{
		{"missing api-key", nil, nil, "", 401, "Unauthorized: Authorization header missing\n"},
		{"invalid api-key", nil, nil, "invalid-key", 401, "Unauthorized: Invalid Authorization header\n"},
		{"valid api-key", nil, nil, validKey1, 200, "Success"},
		{"another valid api-key", nil, nil, validKey2, 200, "Success"},
		{"custom validatorFunc valid key", validator, nil, validKey1, 200, "Success"},
		{"custom validatorFuncWithDB valid key", nil, validatorWithDB, validKey2, 200, "Success"},
		{"custom validatorFuncWithDB in-valid key", nil, validatorWithDB, "invalid-key", 401, "Unauthorized: Invalid Authorization header\n"},
	}

	for i, tc := range testCases {
		rr := httptest.NewRecorder()

		req.Header.Set("X-API-KEY", tc.apiKey)

		provider := APIKeyAuthProvider{
			ValidateFunc:                tc.validatorFunc,
			ValidateFuncWithDatasources: tc.validatorFuncWithDB,
			Container:                   nil,
		}

		wrappedHandler := APIKeyAuthMiddleware(provider, validKey1, validKey2)(testHandler)
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

	provider := APIKeyAuthProvider{}

	wrappedHandler := APIKeyAuthMiddleware(provider)(testHandler)
	wrappedHandler.ServeHTTP(rr, req)

	assert.Equal(t, 200, rr.Code, "TEST Failed.\n")

	assert.Equal(t, "Success", rr.Body.String(), "TEST Failed.\n")
}
