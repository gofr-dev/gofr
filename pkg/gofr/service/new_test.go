package service

import (
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel"
	"go.uber.org/mock/gomock"

	"gofr.dev/pkg/gofr/logging"
)

func validateResponse(t *testing.T, resp *http.Response, err error, hasError bool) {
	t.Helper()

	if resp != nil {
		defer resp.Body.Close()
	}

	if hasError {
		require.Error(t, err)
		assert.Nil(t, resp, "TEST[%d], Failed.\n%s")

		return
	}

	require.NoError(t, err)
	assert.NotNil(t, resp, "TEST[%d], Failed.\n%s")
}

func TestNewHTTPService(t *testing.T) {
	tests := []struct {
		desc           string
		serviceAddress string
	}{
		{"Valid Address", "http://example.com"},
		{"Empty Address", ""},
		{"Invalid Address", "not_a_valid_address"},
	}

	for i, tc := range tests {
		t.Run(tc.desc, func(t *testing.T) {
			service := NewHTTPService(tc.serviceAddress, nil, nil)
			assert.NotNil(t, service, "TEST[%d], Failed.\n%s", i, tc.desc)
		})
	}
}

func TestHTTPService_createAndSendRequest(t *testing.T) {
	ctrl := gomock.NewController(t)
	metrics := NewMockMetrics(ctrl)
	ctx := t.Context()

	tests := []struct {
		desc           string
		queryParams    map[string]any
		body           []byte
		headers        map[string]string
		expQueryParam  string
		expContentType string
	}{
		{"with query params, body and header", map[string]any{"key": "value", "name": []string{"gofr", "test"}},
			[]byte("{Test Body}"), map[string]string{"header1": "value1"}, "key=value&name=gofr&name=test", "application/json"},
		{"with query params, body, header and content type", map[string]any{"key": "value", "name": []string{"gofr", "test"}},
			[]byte("{Test Body}"), map[string]string{"header1": "value1", "content-type": "application/json"},
			"key=value&name=gofr&name=test", "application/json"},
		{"with query params, body, header and content type xml", map[string]any{"key": "value", "name": []string{"gofr", "test"}},
			[]byte("{Test Body}"), map[string]string{"header1": "value1", "content-type": "application/xml"},
			"key=value&name=gofr&name=test", "application/xml"},
		{"without query params, body, header and content type", nil, []byte("{Test Body}"),
			map[string]string{"header1": "value1", "content-type": "application/json"},
			"", "application/json"},
	}

	for i, tc := range tests {
		// Setup a test server
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// read request body
			var body []byte

			body, err := io.ReadAll(r.Body)
			if err != nil {
				t.Fatal("Unable to read request body")
			}

			assert.Equal(t, http.MethodPost, r.Method)
			assert.Equal(t, "/test-path", r.URL.Path)
			assert.Equal(t, tc.expQueryParam, r.URL.RawQuery)
			assert.Contains(t, "value1", r.Header.Get("Header1"))
			assert.Contains(t, tc.expContentType, r.Header.Get("Content-Type"))
			assert.Equal(t, string(tc.body), string(body))

			w.WriteHeader(http.StatusOK)
		}))

		service := &httpService{
			Client:  http.DefaultClient,
			url:     server.URL,
			Tracer:  otel.Tracer("gofr-http-client"),
			Logger:  logging.NewMockLogger(logging.INFO),
			Metrics: metrics,
		}

		metrics.EXPECT().RecordHistogram(gomock.Any(), "app_http_service_response", gomock.Any(), "path", server.URL,
			"method", http.MethodPost, "status", fmt.Sprintf("%v", http.StatusOK)).Times(1)

		resp, err := service.createAndSendRequest(ctx,
			http.MethodPost, "test-path", tc.queryParams, tc.body, tc.headers)
		if err != nil {
			if resp != nil {
				resp.Body.Close()
			}
		}

		require.NoError(t, err)
		assert.NotNil(t, resp, "TEST[%d], Failed.\n%s", i, tc.desc)

		server.Close()
	}
}

func TestHTTPService_Get(t *testing.T) {
	// Setup a test server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodGet, r.Method)
		assert.Equal(t, "/test-path", r.URL.Path)
		assert.Equal(t, "key=value&name=gofr&name=test", r.URL.RawQuery)

		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	service := &httpService{
		Client: http.DefaultClient,
		url:    server.URL,
		Tracer: otel.Tracer("gofr-http-client"),
		Logger: logging.NewMockLogger(logging.INFO),
	}

	// TODO : Nil Correlation ID is coming in logs, it has to be fixed

	resp, err := service.Get(t.Context(), "test-path",
		map[string]any{"key": "value", "name": []string{"gofr", "test"}})

	validateResponse(t, resp, err, false)
}

func TestHTTPService_GetWithHeaders(t *testing.T) {
	// Setup a test server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodGet, r.Method)
		assert.Equal(t, "/test-path", r.URL.Path)
		assert.Equal(t, "key=value&name=gofr&name=test", r.URL.RawQuery)
		assert.Contains(t, "value1", r.Header.Get("Header1"))

		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	service := &httpService{
		Client: http.DefaultClient,
		url:    server.URL,
		Tracer: otel.Tracer("gofr-http-client"),
		Logger: logging.NewMockLogger(logging.INFO),
	}

	// TODO : Nil Correlation ID is coming in logs, it has to be fixed

	resp, err := service.GetWithHeaders(t.Context(), "test-path",
		map[string]any{"key": "value", "name": []string{"gofr", "test"}},
		map[string]string{"header1": "value1"})

	validateResponse(t, resp, err, false)
}

func TestHTTPService_Put(t *testing.T) {
	// Setup a test server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// read request body
		var body []byte

		_, err := r.Body.Read(body)
		if err != nil {
			t.Fatal("Unable to read request body")
		}

		assert.Equal(t, http.MethodPut, r.Method)
		assert.Equal(t, "/test-path", r.URL.Path)
		assert.Equal(t, "key=value&name=gofr&name=test", r.URL.RawQuery)
		assert.Contains(t, "Test Body", string(body))

		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	service := &httpService{
		Client: http.DefaultClient,
		url:    server.URL,
		Tracer: otel.Tracer("gofr-http-client"),
		Logger: logging.NewMockLogger(logging.INFO),
	}

	// TODO : Nil Correlation ID is coming in logs, it has to be fixed

	resp, err := service.Put(t.Context(), "test-path",
		map[string]any{"key": "value", "name": []string{"gofr", "test"}}, []byte("{Test Body}"))

	validateResponse(t, resp, err, false)
}

func TestHTTPService_PutWithHeaders(t *testing.T) {
	// Setup a test server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// read request body
		var body []byte

		_, err := r.Body.Read(body)
		if err != nil {
			t.Fatal("Unable to read request body")
		}

		assert.Equal(t, http.MethodPut, r.Method)
		assert.Equal(t, "/test-path", r.URL.Path)
		assert.Equal(t, "key=value&name=gofr&name=test", r.URL.RawQuery)
		assert.Contains(t, "value1", r.Header.Get("Header1"))
		assert.Contains(t, "Test Body", string(body))

		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	service := &httpService{
		Client: http.DefaultClient,
		url:    server.URL,
		Tracer: otel.Tracer("gofr-http-client"),
		Logger: logging.NewMockLogger(logging.INFO),
	}

	// TODO : Nil Correlation ID is coming in logs, it has to be fixed

	resp, err := service.PutWithHeaders(t.Context(), "test-path",
		map[string]any{"key": "value", "name": []string{"gofr", "test"}}, []byte("{Test Body}"),
		map[string]string{"header1": "value1"})

	validateResponse(t, resp, err, false)
}

func TestHTTPService_Patch(t *testing.T) {
	// Setup a test server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// read request body
		var body []byte

		_, err := r.Body.Read(body)
		if err != nil {
			t.Fatal("Unable to read request body")
		}

		assert.Equal(t, http.MethodPatch, r.Method)
		assert.Equal(t, "/test-path", r.URL.Path)
		assert.Equal(t, "key=value&name=gofr&name=test", r.URL.RawQuery)
		assert.Contains(t, "Test Body", string(body))

		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	service := &httpService{
		Client: http.DefaultClient,
		url:    server.URL,
		Tracer: otel.Tracer("gofr-http-client"),
		Logger: logging.NewMockLogger(logging.INFO),
	}

	// TODO : Nil Correlation ID is coming in logs, it has to be fixed

	resp, err := service.Patch(t.Context(), "test-path",
		map[string]any{"key": "value", "name": []string{"gofr", "test"}}, []byte("{Test Body}"))

	validateResponse(t, resp, err, false)
}

func TestHTTPService_PatchWithHeaders(t *testing.T) {
	// Setup a test server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// read request body
		var body []byte

		_, err := r.Body.Read(body)
		if err != nil {
			t.Fatal("Unable to read request body")
		}

		assert.Equal(t, http.MethodPatch, r.Method)
		assert.Equal(t, "/test-path", r.URL.Path)
		assert.Equal(t, "key=value&name=gofr&name=test", r.URL.RawQuery)
		assert.Contains(t, "value1", r.Header.Get("Header1"))
		assert.Contains(t, "Test Body", string(body))

		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	service := &httpService{
		Client: http.DefaultClient,
		url:    server.URL,
		Tracer: otel.Tracer("gofr-http-client"),
		Logger: logging.NewMockLogger(logging.INFO),
	}

	// TODO : Nil Correlation ID is coming in logs, it has to be fixed

	resp, err := service.PatchWithHeaders(t.Context(), "test-path",
		map[string]any{"key": "value", "name": []string{"gofr", "test"}}, []byte("{Test Body}"),
		map[string]string{"header1": "value1"})

	validateResponse(t, resp, err, false)
}

func TestHTTPService_Post(t *testing.T) {
	// Setup a test server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// read request body
		var body []byte

		_, err := r.Body.Read(body)
		if err != nil {
			t.Fatal("Unable to read request body")
		}

		assert.Equal(t, http.MethodPost, r.Method)
		assert.Equal(t, "/test-path", r.URL.Path)
		assert.Equal(t, "key=value&name=gofr&name=test", r.URL.RawQuery)
		assert.Contains(t, "Test Body", string(body))

		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	service := &httpService{
		Client: http.DefaultClient,
		url:    server.URL,
		Tracer: otel.Tracer("gofr-http-client"),
		Logger: logging.NewMockLogger(logging.INFO),
	}

	// TODO : Nil Correlation ID is coming in logs, it has to be fixed

	resp, err := service.Post(t.Context(), "test-path",
		map[string]any{"key": "value", "name": []string{"gofr", "test"}}, []byte("{Test Body}"))

	validateResponse(t, resp, err, false)
}

func TestHTTPService_PostWithHeaders(t *testing.T) {
	// Setup a test server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// read request body
		var body []byte

		_, err := r.Body.Read(body)
		if err != nil {
			t.Fatal("Unable to read request body")
		}

		assert.Equal(t, http.MethodPost, r.Method)
		assert.Equal(t, "/test-path", r.URL.Path)
		assert.Equal(t, "key=value&name=gofr&name=test", r.URL.RawQuery)
		assert.Contains(t, "value1", r.Header.Get("Header1"))
		assert.Contains(t, "Test Body", string(body))

		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	service := &httpService{
		Client: http.DefaultClient,
		url:    server.URL,
		Tracer: otel.Tracer("gofr-http-client"),
		Logger: logging.NewMockLogger(logging.INFO),
	}

	// TODO : Nil Correlation ID is coming in logs, it has to be fixed

	resp, err := service.PostWithHeaders(t.Context(), "test-path",
		map[string]any{"key": "value", "name": []string{"gofr", "test"}}, []byte("{Test Body}"),
		map[string]string{"header1": "value1"})

	validateResponse(t, resp, err, false)
}

func TestHTTPService_Delete(t *testing.T) {
	// Setup a test server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// read request body
		var body []byte

		_, err := r.Body.Read(body)
		if err != nil {
			t.Fatal("Unable to read request body")
		}

		assert.Equal(t, http.MethodDelete, r.Method)
		assert.Equal(t, "/test-path", r.URL.Path)
		assert.Contains(t, "Test Body", string(body))

		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	service := &httpService{
		Client: http.DefaultClient,
		url:    server.URL,
		Tracer: otel.Tracer("gofr-http-client"),
		Logger: logging.NewMockLogger(logging.INFO),
	}

	// TODO : Nil Correlation ID is coming in logs, it has to be fixed

	resp, err := service.Delete(t.Context(), "test-path", []byte("{Test Body}"))

	validateResponse(t, resp, err, false)
}

func TestHTTPService_DeleteWithHeaders(t *testing.T) {
	// Setup a test server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// read request body
		var body []byte

		_, err := r.Body.Read(body)
		if err != nil {
			t.Fatal("Unable to read request body")
		}

		assert.Equal(t, http.MethodDelete, r.Method)
		assert.Equal(t, "/test-path", r.URL.Path)
		assert.Contains(t, "value1", r.Header.Get("Header1"))
		assert.Contains(t, "Test Body", string(body))

		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	service := &httpService{
		Client: http.DefaultClient,
		url:    server.URL,
		Tracer: otel.Tracer("gofr-http-client"),
		Logger: logging.NewMockLogger(logging.INFO),
	}

	// TODO : Nil Correlation ID is coming in logs, it has to be fixed

	resp, err := service.DeleteWithHeaders(t.Context(), "test-path", []byte("{Test Body}"),
		map[string]string{"header1": "value1"})

	validateResponse(t, resp, err, false)
}

func TestHTTPService_createAndSendRequestCreateRequestFailure(t *testing.T) {
	service := &httpService{
		Client: http.DefaultClient,
		Tracer: otel.Tracer("gofr-http-client"),
		Logger: logging.NewMockLogger(logging.INFO),
	}

	ctx := t.Context()
	// when params value is of type []string then last value is sent in request
	resp, err := service.createAndSendRequest(ctx,
		"!@#$", "test-path", map[string]any{"key": "value", "name": []string{"gofr", "test"}},
		[]byte("{Test Body}"), map[string]string{"header1": "value1"})

	validateResponse(t, resp, err, true)
}

func TestHTTPService_createAndSendRequestServerError(t *testing.T) {
	ctrl := gomock.NewController(t)
	metrics := NewMockMetrics(ctrl)

	service := &httpService{
		Client:  http.DefaultClient,
		Tracer:  otel.Tracer("gofr-http-client"),
		Logger:  logging.NewMockLogger(logging.INFO),
		Metrics: metrics,
	}

	ctx := t.Context()

	metrics.EXPECT().RecordHistogram(gomock.Any(), "app_http_service_response", gomock.Any(), "path", gomock.Any(),
		"method", http.MethodPost, "status", fmt.Sprintf("%v", http.StatusInternalServerError))

	// when params value is of type []string then last value is sent in request
	resp, err := service.createAndSendRequest(ctx,
		http.MethodPost, "test-path", map[string]any{"key": "value", "name": []string{"gofr", "test"}},
		[]byte("{Test Body}"), map[string]string{"header1": "value1"})

	validateResponse(t, resp, err, true)
}
