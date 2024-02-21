package service

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/otel"

	"gofr.dev/pkg/gofr/testutil"
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
		Logger: testutil.NewMockLogger(testutil.INFOLOG),
	}

	// TODO : Nil Correlation ID is coming in logs, it has to be fixed

	resp, err := service.Get(context.Background(), "test-path",
		map[string]interface{}{"key": "value", "name": []string{"gofr", "test"}})

	assert.NoError(t, err)
	assert.NotNil(t, resp, "TEST, Failed.")

	defer resp.Body.Close()

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
		Logger: testutil.NewMockLogger(testutil.INFOLOG),
	}

	// TODO : Nil Correlation ID is coming in logs, it has to be fixed

	resp, err := service.GetWithHeaders(context.Background(), "test-path",
		map[string]interface{}{"key": "value", "name": []string{"gofr", "test"}},
		map[string]string{"header1": "value1"})

	assert.NoError(t, err)
	assert.NotNil(t, resp, "TEST, Failed.")

	defer resp.Body.Close()

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
		Logger: testutil.NewMockLogger(testutil.INFOLOG),
	}

	// TODO : Nil Correlation ID is coming in logs, it has to be fixed

	resp, err := service.Put(context.Background(), "test-path",
		map[string]interface{}{"key": "value", "name": []string{"gofr", "test"}}, []byte("{Test Body}"))

	assert.NoError(t, err)
	assert.NotNil(t, resp, "TEST, Failed.")

	defer resp.Body.Close()
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
		Logger: testutil.NewMockLogger(testutil.INFOLOG),
	}

	// TODO : Nil Correlation ID is coming in logs, it has to be fixed

	resp, err := service.PutWithHeaders(context.Background(), "test-path",
		map[string]interface{}{"key": "value", "name": []string{"gofr", "test"}}, []byte("{Test Body}"),
		map[string]string{"header1": "value1"})

	assert.NoError(t, err)
	assert.NotNil(t, resp, "TEST, Failed.")

	defer resp.Body.Close()
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
		Logger: testutil.NewMockLogger(testutil.INFOLOG),
	}

	// TODO : Nil Correlation ID is coming in logs, it has to be fixed

	resp, err := service.Patch(context.Background(), "test-path",
		map[string]interface{}{"key": "value", "name": []string{"gofr", "test"}}, []byte("{Test Body}"))

	assert.NoError(t, err)
	assert.NotNil(t, resp, "TEST, Failed.")

	defer resp.Body.Close()
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
		Logger: testutil.NewMockLogger(testutil.INFOLOG),
	}

	// TODO : Nil Correlation ID is coming in logs, it has to be fixed

	resp, err := service.PutWithHeaders(context.Background(), "test-path",
		map[string]interface{}{"key": "value", "name": []string{"gofr", "test"}}, []byte("{Test Body}"),
		map[string]string{"header1": "value1"})

	assert.NoError(t, err)
	assert.NotNil(t, resp, "TEST, Failed.")

	defer resp.Body.Close()
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
		Logger: testutil.NewMockLogger(testutil.INFOLOG),
	}

	// TODO : Nil Correlation ID is coming in logs, it has to be fixed

	resp, err := service.Post(context.Background(), "test-path",
		map[string]interface{}{"key": "value", "name": []string{"gofr", "test"}}, []byte("{Test Body}"))

	assert.NoError(t, err)
	assert.NotNil(t, resp, "TEST, Failed.")

	defer resp.Body.Close()
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
		Logger: testutil.NewMockLogger(testutil.INFOLOG),
	}

	// TODO : Nil Correlation ID is coming in logs, it has to be fixed

	resp, err := service.PostWithHeaders(context.Background(), "test-path",
		map[string]interface{}{"key": "value", "name": []string{"gofr", "test"}}, []byte("{Test Body}"),
		map[string]string{"header1": "value1"})

	assert.NoError(t, err)
	assert.NotNil(t, resp, "TEST, Failed.")

	defer resp.Body.Close()
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
		Logger: testutil.NewMockLogger(testutil.INFOLOG),
	}

	// TODO : Nil Correlation ID is coming in logs, it has to be fixed

	resp, err := service.Delete(context.Background(), "test-path", []byte("{Test Body}"))

	assert.NoError(t, err)
	assert.NotNil(t, resp, "TEST, Failed.")

	defer resp.Body.Close()
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
		Logger: testutil.NewMockLogger(testutil.INFOLOG),
	}

	// TODO : Nil Correlation ID is coming in logs, it has to be fixed

	resp, err := service.DeleteWithHeaders(context.Background(), "test-path", []byte("{Test Body}"),
		map[string]string{"header1": "value1"})

	assert.NoError(t, err)
	assert.NotNil(t, resp, "TEST, Failed.")

	defer resp.Body.Close()
}

func TestHTTPService_createAndSendRequest(t *testing.T) {
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
		Client: http.DefaultClient,
		url:    server.URL,
		Tracer: otel.Tracer("gofr-http-client"),
		Logger: testutil.NewMockLogger(testutil.INFOLOG),
	}

	ctx := context.Background()
	// when params value is of type []string then last value is sent in request
	resp, err := service.createAndSendRequest(ctx,
		http.MethodPost, "test-path", map[string]interface{}{"key": "value", "name": []string{"gofr", "test"}},
		[]byte("{Test Body}"), map[string]string{"header1": "value1"})

	if err != nil {
		defer resp.Body.Close()
	}

	assert.NoError(t, err)
	assert.NotNil(t, resp, "TEST[%d], Failed.\n%s")
}
