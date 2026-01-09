package service

import (
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestMain(m *testing.M) {
	os.Setenv("GOFR_TELEMETRY", "false")
	m.Run()
}

func TestNewAPIKeyConfig(t *testing.T) {
	testCases := []struct {
		apiKey       string
		apiKeyOption Options
		err          error
	}{
		{apiKey: "valid", apiKeyOption: &APIKeyConfig{APIKey: "valid"}},
		{apiKey: "  valid  ", apiKeyOption: &APIKeyConfig{APIKey: "valid"}},
		{apiKey: "", err: AuthErr{Message: "non empty api key is required"}},
		{apiKey: "  ", err: AuthErr{Message: "non empty api key is required"}},
	}

	for i, tc := range testCases {
		options, err := NewAPIKeyConfig(tc.apiKey)
		assert.Equal(t, tc.apiKeyOption, options, "failed test case #%d", i)
		assert.Equal(t, tc.err, err, "failed test case #%d", i)
	}
}

func TestAddAuthorizationHeader_APIKey(t *testing.T) {
	testCases := []struct {
		apiKey   string
		headers  map[string]string
		response map[string]string
		err      error
	}{
		{
			apiKey:   "valid",
			response: map[string]string{xAPIKeyHeader: "valid"},
		},
		{
			apiKey:   "valid",
			headers:  map[string]string{xAPIKeyHeader: "existing-value"},
			response: map[string]string{xAPIKeyHeader: "existing-value"},
			err:      AuthErr{Message: `value existing-value already exists for header X-Api-Key`},
		},
		{
			apiKey:   "valid",
			headers:  map[string]string{"header-key": "existing-value"},
			response: map[string]string{"header-key": "existing-value", xAPIKeyHeader: "valid"},
		},
	}
	for i, tc := range testCases {
		config := APIKeyConfig{APIKey: tc.apiKey}
		response, err := config.addAuthorizationHeader(t.Context(), tc.headers)
		assert.Equal(t, tc.response, response, "failed test case #%d", i)
		assert.Equal(t, tc.err, err, "failed test case #%d", i)
	}
}

func setupAPIKeyAuthHTTPServer(t *testing.T, config *APIKeyConfig) *httptest.Server {
	t.Helper()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		statusCode := http.StatusOK
		if r.Header.Get(xAPIKeyHeader) != config.APIKey {
			statusCode = http.StatusUnauthorized
		}

		w.WriteHeader(statusCode)
	}))

	t.Cleanup(func() {
		server.Close()
	})

	return server
}
