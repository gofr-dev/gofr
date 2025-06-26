package service

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"gofr.dev/pkg/gofr/logging"
)

func TestMain(m *testing.M) {
	os.Setenv("GOFR_TELEMETRY", "false")
	m.Run()
}

func TestSetXApiKey(t *testing.T) {
	testCases := []struct {
		name           string
		headers        map[string]string
		apiKey         string
		expectedHeader map[string]string
	}{
		{
			name:           "existing header empty",
			headers:        nil,
			apiKey:         "valid-key",
			expectedHeader: map[string]string{apiKeyHeader: "valid-key"},
		},
		{
			name:           "existing header non empty",
			headers:        map[string]string{"header": "value"},
			apiKey:         "valid-key",
			expectedHeader: map[string]string{"header": "value", apiKeyHeader: "valid-key"},
		},

		{
			name:           "existing header containing api key",
			headers:        map[string]string{apiKeyHeader: "value"},
			apiKey:         "valid-key",
			expectedHeader: map[string]string{apiKeyHeader: "valid-key"},
		},
	}

	for i, tc := range testCases {
		result := setXApiKey(tc.headers, tc.apiKey)
		assert.Equal(t, tc.expectedHeader, result, "failed test case #%d: %v", i, tc.name)
	}
}

func TestNewAPIKeyConfig(t *testing.T) {
	testCases := []struct {
		apiKey       string
		apiKeyOption Options
		err          error
	}{
		{apiKey: "valid", apiKeyOption: &APIKeyConfig{APIKey: "valid"}, err: nil},
		{apiKey: "", apiKeyOption: nil, err: AuthErr{Message: "non empty api key is required"}},
		{apiKey: "  ", apiKeyOption: nil, err: AuthErr{Message: "non empty api key is required"}},
	}

	for i, tc := range testCases {
		options, err := NewAPIKeyConfig(tc.apiKey)
		assert.Equal(t, tc.apiKeyOption, options, "failed test case #%d", i)
		assert.Equal(t, tc.err, err, "failed test case #%d", i)
	}
}

func TestAPIKeyAuthProvider(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	testCases := []struct {
		httpMethod string
		statusCode int
	}{

		{httpMethod: http.MethodPost, statusCode: http.StatusCreated},
		{httpMethod: http.MethodGet, statusCode: http.StatusOK},
		{httpMethod: http.MethodDelete, statusCode: http.StatusNoContent},
		{httpMethod: http.MethodPatch, statusCode: http.StatusOK},
		{httpMethod: http.MethodPut, statusCode: http.StatusOK},
	}

	path := "/path"
	queryParams := map[string]any{"key": "value"}
	body := []byte("body")

	for i, tc := range testCases {
		t.Run(fmt.Sprintf("Test Case #%d", i), func(t *testing.T) {
			server := httpServerSetup(t, tc.httpMethod, tc.statusCode)

			httpService := NewHTTPService(server.URL, logging.NewMockLogger(logging.INFO), nil,
				&APIKeyConfig{"valid-key"})

			var (
				resp *http.Response
				err  error
			)

			switch tc.httpMethod {
			case http.MethodGet:
				resp, err = httpService.Get(t.Context(), path, queryParams)
			case http.MethodPost:
				resp, err = httpService.Post(t.Context(), path, queryParams, body)
			case http.MethodPut:
				resp, err = httpService.Put(t.Context(), path, queryParams, body)
			case http.MethodPatch:
				resp, err = httpService.Patch(t.Context(), path, queryParams, body)
			case http.MethodDelete:
				resp, err = httpService.Delete(t.Context(), path, body)
			}

			require.NoError(t, err)

			assert.Equal(t, tc.statusCode, resp.StatusCode)
			require.NoError(t, err)

			err = resp.Body.Close()
			if err != nil {
				t.Errorf("error closing response body %v", err)
			}
		})
	}
}

func httpServerSetup(t *testing.T, method string, responseCode int) *httptest.Server {
	t.Helper()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, method, r.Method)

		w.WriteHeader(responseCode)
	}))

	t.Cleanup(func() {
		server.Close()
	})

	return server
}
