package middleware

import (
	"bytes"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestExtractIP(t *testing.T) {
	tests := []struct {
		name          string
		trustProxies  bool
		remoteAddr    string
		headers       map[string]string
		expectedKey   string
		expectedError bool
	}{
		{
			name:         "Extract from RemoteAddr",
			trustProxies: false,
			remoteAddr:   "192.168.1.1:12345",
			expectedKey:  "192.168.1.1",
		},
		{
			name:         "Extract from X-Forwarded-For with trust",
			trustProxies: true,
			remoteAddr:   "10.0.0.1:12345",
			headers: map[string]string{
				"X-Forwarded-For": "203.0.113.1, 198.51.100.1",
			},
			expectedKey: "203.0.113.1",
		},
		{
			name:         "Ignore X-Forwarded-For without trust",
			trustProxies: false,
			remoteAddr:   "10.0.0.1:12345",
			headers: map[string]string{
				"X-Forwarded-For": "203.0.113.1",
			},
			expectedKey: "10.0.0.1",
		},
		{
			name:         "Empty IP returns unknown",
			trustProxies: false,
			remoteAddr:   "",
			expectedKey:  "unknown",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/test", http.NoBody)
			req.RemoteAddr = tt.remoteAddr
			for k, v := range tt.headers {
				req.Header.Set(k, v)
			}

			extractor := ExtractIP(tt.trustProxies)
			key, err := extractor(req)

			if tt.expectedError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.expectedKey, key)
			}
		})
	}
}

func TestExtractHeader(t *testing.T) {
	tests := []struct {
		name          string
		headerName    string
		headerValue   string
		expectedKey   string
		expectedError error
	}{
		{
			name:        "Extract API key from header",
			headerName:  "X-API-Key",
			headerValue: "secret-key-123",
			expectedKey: "secret-key-123",
		},
		{
			name:          "Missing header returns error",
			headerName:    "X-API-Key",
			headerValue:   "",
			expectedError: ErrKeyNotFound,
		},
		{
			name:        "Extract custom header",
			headerName:  "X-Tenant-ID",
			headerValue: "tenant-456",
			expectedKey: "tenant-456",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/test", http.NoBody)
			if tt.headerValue != "" {
				req.Header.Set(tt.headerName, tt.headerValue)
			}

			extractor := ExtractHeader(tt.headerName)
			key, err := extractor(req)

			if tt.expectedError != nil {
				assert.ErrorIs(t, err, tt.expectedError)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.expectedKey, key)
			}
		})
	}
}

func TestExtractParam(t *testing.T) {
	tests := []struct {
		name          string
		url           string
		paramName     string
		expectedKey   string
		expectedError error
	}{
		{
			name:        "Extract user_id from query",
			url:         "/api/user?user_id=12345",
			paramName:   "user_id",
			expectedKey: "12345",
		},
		{
			name:          "Missing param returns error",
			url:           "/api/user",
			paramName:     "user_id",
			expectedError: ErrKeyNotFound,
		},
		{
			name:        "Extract email from query",
			url:         "/login?email=user@example.com",
			paramName:   "email",
			expectedKey: "user@example.com",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, tt.url, http.NoBody)

			extractor := ExtractParam(tt.paramName)
			key, err := extractor(req)

			if tt.expectedError != nil {
				assert.ErrorIs(t, err, tt.expectedError)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.expectedKey, key)
			}
		})
	}
}

func TestExtractBody(t *testing.T) {
	tests := []struct {
		name          string
		body          string
		contentType   string
		field         string
		expectedKey   string
		expectedError error
	}{
		{
			name:        "Extract email from JSON body",
			body:        `{"email":"user@example.com","password":"secret"}`,
			contentType: "application/json",
			field:       "email",
			expectedKey: "user@example.com",
		},
		{
			name:        "Extract user_id from JSON body",
			body:        `{"user_id":"12345","action":"verify"}`,
			contentType: "application/json",
			field:       "user_id",
			expectedKey: "12345",
		},
		{
			name:          "Missing field returns error",
			body:          `{"password":"secret"}`,
			contentType:   "application/json",
			field:         "email",
			expectedError: ErrKeyNotFound,
		},
		{
			name:          "Invalid JSON returns error",
			body:          `{invalid json}`,
			contentType:   "application/json",
			field:         "email",
			expectedError: ErrInvalidBody,
		},
		{
			name:          "Non-JSON content type returns error",
			body:          "email=user@example.com",
			contentType:   "application/x-www-form-urlencoded",
			field:         "email",
			expectedError: ErrInvalidBody,
		},
		{
			name:          "Empty field value returns error",
			body:          `{"email":""}`,
			contentType:   "application/json",
			field:         "email",
			expectedError: ErrKeyNotFound,
		},
		{
			name:          "Non-string field returns error",
			body:          `{"count":42}`,
			contentType:   "application/json",
			field:         "count",
			expectedError: ErrKeyNotFound,
		},
		{
			name:          "Body too large returns error",
			body:          strings.Repeat("x", 1024*1024+1), // 1MB + 1 byte
			contentType:   "application/json",
			field:         "email",
			expectedError: ErrBodyTooLarge,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodPost, "/test", bytes.NewBufferString(tt.body))
			req.Header.Set("Content-Type", tt.contentType)

			extractor := ExtractBody(tt.field)
			key, err := extractor(req)

			if tt.expectedError != nil {
				assert.ErrorIs(t, err, tt.expectedError)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.expectedKey, key)
			}

			// Verify body can still be read after extraction
			if tt.expectedError == nil {
				body, err := io.ReadAll(req.Body)
				require.NoError(t, err)
				assert.Equal(t, tt.body, string(body))
			}
		})
	}
}

func TestExtractCombined(t *testing.T) {
	tests := []struct {
		name          string
		extractors    []KeyExtractor
		headers       map[string]string
		url           string
		expectedKey   string
		expectedError error
	}{
		{
			name: "First extractor succeeds",
			extractors: []KeyExtractor{
				ExtractHeader("X-API-Key"),
				ExtractParam("user_id"),
			},
			headers: map[string]string{
				"X-API-Key": "key-123",
			},
			url:         "/test?user_id=456",
			expectedKey: "key-123",
		},
		{
			name: "Fallback to second extractor",
			extractors: []KeyExtractor{
				ExtractHeader("X-API-Key"),
				ExtractParam("user_id"),
			},
			url:         "/test?user_id=456",
			expectedKey: "456",
		},
		{
			name: "All extractors fail",
			extractors: []KeyExtractor{
				ExtractHeader("X-API-Key"),
				ExtractParam("user_id"),
			},
			url:           "/test",
			expectedError: ErrKeyNotFound,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, tt.url, http.NoBody)
			for k, v := range tt.headers {
				req.Header.Set(k, v)
			}

			extractor := ExtractCombined(tt.extractors...)
			key, err := extractor(req)

			if tt.expectedError != nil {
				assert.ErrorIs(t, err, tt.expectedError)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.expectedKey, key)
			}
		})
	}
}

func TestExtractCombined_NoPanic(t *testing.T) {
	t.Run("Panics when no extractors provided", func(t *testing.T) {
		assert.Panics(t, func() {
			ExtractCombined()
		})
	})
}

func TestExtractStatic(t *testing.T) {
	extractor := ExtractStatic("global-limit")
	req := httptest.NewRequest(http.MethodGet, "/test", http.NoBody)

	key, err := extractor(req)

	assert.NoError(t, err)
	assert.Equal(t, "global-limit", key)
}

func TestRateLimiterWithKeyExtractor(t *testing.T) {
	tests := []struct {
		name          string
		config        RateLimiterConfig
		setupRequest  func() *http.Request
		expectedAllow bool
		requestCount  int
	}{
		{
			name: "Rate limit by API key",
			config: RateLimiterConfig{
				RequestsPerSecond: 2,
				Burst:             2,
				KeyExtractor:      ExtractHeader("X-API-Key"),
			},
			setupRequest: func() *http.Request {
				req := httptest.NewRequest(http.MethodGet, "/test", http.NoBody)
				req.Header.Set("X-API-Key", "test-key-1")
				return req
			},
			requestCount: 3, // 3rd request should be blocked
		},
		{
			name: "Rate limit by email in body",
			config: RateLimiterConfig{
				RequestsPerSecond: 1,
				Burst:             1,
				KeyExtractor:      ExtractBody("email"),
			},
			setupRequest: func() *http.Request {
				body := `{"email":"user@test.com"}`
				req := httptest.NewRequest(http.MethodPost, "/login", strings.NewReader(body))
				req.Header.Set("Content-Type", "application/json")
				return req
			},
			requestCount: 2, // 2nd request should be blocked
		},
		{
			name: "Backward compatibility with PerIP",
			config: RateLimiterConfig{
				RequestsPerSecond: 2,
				Burst:             2,
				PerIP:             true,
			},
			setupRequest: func() *http.Request {
				req := httptest.NewRequest(http.MethodGet, "/test", http.NoBody)
				req.RemoteAddr = "192.168.1.1:12345"
				return req
			},
			requestCount: 3, // 3rd request should be blocked
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			metrics := newRateLimiterMockMetrics()
			handler := RateLimiter(tt.config, metrics)(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(http.StatusOK)
			}))

			var lastStatus int
			for i := 0; i < tt.requestCount; i++ {
				req := tt.setupRequest()
				rr := httptest.NewRecorder()
				handler.ServeHTTP(rr, req)
				lastStatus = rr.Code
			}

			// Last request should be rate limited
			assert.Equal(t, http.StatusTooManyRequests, lastStatus)
		})
	}
}

func TestRateLimiterKeyExtractionFailure(t *testing.T) {
	metrics := newRateLimiterMockMetrics()
	config := RateLimiterConfig{
		RequestsPerSecond: 10,
		Burst:             10,
		KeyExtractor:      ExtractHeader("X-API-Key"), // Header not present
	}

	handler := RateLimiter(config, metrics)(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/test", http.NoBody)
	req.RemoteAddr = "192.168.1.1:12345"
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	// Should fallback to IP and succeed
	assert.Equal(t, http.StatusOK, rr.Code)
	
	// Should log key extraction failure
	assert.Equal(t, 1, metrics.GetCounter("app_http_rate_limit_key_extraction_failed"))
}
