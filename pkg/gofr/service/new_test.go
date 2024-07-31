package service

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel"
	"go.uber.org/mock/gomock"

	"gofr.dev/pkg/gofr/logging"
)

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

	// Setup a test server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// read request body
		var body []byte

		_, err := r.Body.Read(body)
		if err != nil {
			t.Fatal("Unable to read request body")
		}

		assert.Equal(t, "POST", r.Method)
		assert.Equal(t, "/test-path", r.URL.Path)
		assert.Equal(t, "key=value&name=test", r.URL.RawQuery)
		assert.Contains(t, "value1", r.Header.Get("header1"))
		assert.Contains(t, "Test Body", string(body))

		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	service := &httpService{
		Client:  http.DefaultClient,
		url:     server.URL,
		Tracer:  otel.Tracer("gofr-http-client"),
		Logger:  logging.NewMockLogger(logging.INFO),
		Metrics: metrics,
	}

	ctx := context.Background()

	metrics.EXPECT().RecordHistogram(ctx, "app_http_service_response", gomock.Any(), "path", server.URL,
		"method", http.MethodPost, "status", fmt.Sprintf("%v", http.StatusOK))

	// when params value is of type []string then last value is sent in request
	resp, err := service.createAndSendRequest(ctx,
		http.MethodPost, "test-path", map[string]interface{}{"key": "value", "name": []string{"gofr", "test"}},
		[]byte("{Test Body}"), map[string]string{"header1": "value1"})

	if err != nil {
		if resp != nil {
			defer resp.Body.Close()
		}
	}

	require.NoError(t, err)
	assert.NotNil(t, resp, "TEST[%d], Failed.\n%s")
}

func TestHTTPService_Get(t *testing.T) {
	// Setup a test server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodGet, r.Method)
		assert.Equal(t, "/test-path", r.URL.Path)
		assert.Equal(t, "key=value&name=test", r.URL.RawQuery)

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

	resp, err := service.Get(context.Background(), "test-path",
		map[string]interface{}{"key": "value", "name": []string{"gofr", "test"}})

	if resp != nil {
		defer resp.Body.Close()
	}

	require.NoError(t, err)
	assert.NotNil(t, resp, "TEST, Failed.")
}

func TestHTTPService_GetWithHeaders(t *testing.T) {
	// Setup a test server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodGet, r.Method)
		assert.Equal(t, "/test-path", r.URL.Path)
		assert.Equal(t, "key=value&name=test", r.URL.RawQuery)
		assert.Contains(t, "value1", r.Header.Get("header1"))

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

	resp, err := service.GetWithHeaders(context.Background(), "test-path",
		map[string]interface{}{"key": "value", "name": []string{"gofr", "test"}},
		map[string]string{"header1": "value1"})

	if resp != nil {
		defer resp.Body.Close()
	}

	require.NoError(t, err)
	assert.NotNil(t, resp, "TEST, Failed.")
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
		assert.Equal(t, "key=value&name=test", r.URL.RawQuery)
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

	resp, err := service.Put(context.Background(), "test-path",
		map[string]interface{}{"key": "value", "name": []string{"gofr", "test"}}, []byte("{Test Body}"))

	if resp != nil {
		defer resp.Body.Close()
	}

	require.NoError(t, err)
	assert.NotNil(t, resp, "TEST, Failed.")
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
		assert.Equal(t, "key=value&name=test", r.URL.RawQuery)
		assert.Contains(t, "value1", r.Header.Get("header1"))
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

	resp, err := service.PutWithHeaders(context.Background(), "test-path",
		map[string]interface{}{"key": "value", "name": []string{"gofr", "test"}}, []byte("{Test Body}"),
		map[string]string{"header1": "value1"})

	if resp != nil {
		defer resp.Body.Close()
	}

	require.NoError(t, err)
	assert.NotNil(t, resp, "TEST, Failed.")
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
		assert.Equal(t, "key=value&name=test", r.URL.RawQuery)
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

	resp, err := service.Patch(context.Background(), "test-path",
		map[string]interface{}{"key": "value", "name": []string{"gofr", "test"}}, []byte("{Test Body}"))

	if resp != nil {
		defer resp.Body.Close()
	}

	require.NoError(t, err)
	assert.NotNil(t, resp, "TEST, Failed.")
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

		assert.Equal(t, http.MethodPut, r.Method)
		assert.Equal(t, "/test-path", r.URL.Path)
		assert.Equal(t, "key=value&name=test", r.URL.RawQuery)
		assert.Contains(t, "value1", r.Header.Get("header1"))
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

	resp, err := service.PutWithHeaders(context.Background(), "test-path",
		map[string]interface{}{"key": "value", "name": []string{"gofr", "test"}}, []byte("{Test Body}"),
		map[string]string{"header1": "value1"})

	if resp != nil {
		defer resp.Body.Close()
	}

	require.NoError(t, err)
	assert.NotNil(t, resp, "TEST, Failed.")
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
		assert.Equal(t, "key=value&name=test", r.URL.RawQuery)
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

	resp, err := service.Post(context.Background(), "test-path",
		map[string]interface{}{"key": "value", "name": []string{"gofr", "test"}}, []byte("{Test Body}"))

	if resp != nil {
		defer resp.Body.Close()
	}

	require.NoError(t, err)
	assert.NotNil(t, resp, "TEST, Failed.")
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
		assert.Equal(t, "key=value&name=test", r.URL.RawQuery)
		assert.Contains(t, "value1", r.Header.Get("header1"))
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

	resp, err := service.PostWithHeaders(context.Background(), "test-path",
		map[string]interface{}{"key": "value", "name": []string{"gofr", "test"}}, []byte("{Test Body}"),
		map[string]string{"header1": "value1"})

	if resp != nil {
		defer resp.Body.Close()
	}

	require.NoError(t, err)
	assert.NotNil(t, resp, "TEST, Failed.")
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

	resp, err := service.Delete(context.Background(), "test-path", []byte("{Test Body}"))

	if resp != nil {
		defer resp.Body.Close()
	}

	require.NoError(t, err)
	assert.NotNil(t, resp, "TEST, Failed.")
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
		assert.Contains(t, "value1", r.Header.Get("header1"))
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

	resp, err := service.DeleteWithHeaders(context.Background(), "test-path", []byte("{Test Body}"),
		map[string]string{"header1": "value1"})

	if resp != nil {
		defer resp.Body.Close()
	}

	require.NoError(t, err)
	assert.NotNil(t, resp, "TEST, Failed.")
}

func TestHTTPService_createAndSendRequestCreateRequestFailure(t *testing.T) {
	service := &httpService{
		Client: http.DefaultClient,
		Tracer: otel.Tracer("gofr-http-client"),
		Logger: logging.NewMockLogger(logging.INFO),
	}

	ctx := context.Background()
	// when params value is of type []string then last value is sent in request
	resp, err := service.createAndSendRequest(ctx,
		"!@#$", "test-path", map[string]interface{}{"key": "value", "name": []string{"gofr", "test"}},
		[]byte("{Test Body}"), map[string]string{"header1": "value1"})

	if resp != nil {
		defer resp.Body.Close()
	}

	require.Error(t, err)
	assert.Nil(t, resp, "TEST[%d], Failed.\n%s")
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

	ctx := context.Background()

	metrics.EXPECT().RecordHistogram(ctx, "app_http_service_response", gomock.Any(), "path", gomock.Any(),
		"method", http.MethodPost, "status", fmt.Sprintf("%v", http.StatusInternalServerError))

	// when params value is of type []string then last value is sent in request
	resp, err := service.createAndSendRequest(ctx,
		http.MethodPost, "test-path", map[string]interface{}{"key": "value", "name": []string{"gofr", "test"}},
		[]byte("{Test Body}"), map[string]string{"header1": "value1"})

	if resp != nil {
		defer resp.Body.Close()
	}

	require.Error(t, err)
	assert.Nil(t, resp, "TEST[%d], Failed.\n%s")
}
