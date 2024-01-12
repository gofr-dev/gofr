package service

import (
	"context"
	"go.opentelemetry.io/otel"
	"gofr.dev/pkg/gofr"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
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
			service := NewHTTPService(tc.serviceAddress)
			assert.NotNil(t, service, "TEST[%d], Failed.\n%s", i, tc.desc)
		})
	}
}

func TestHTTPService_Get(t *testing.T) {
	// Setup a test server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	service := &httpService{
		Client: http.DefaultClient,
		url:    server.URL,
		Tracer: otel.Tracer("gofr-http-client"),
	}

	tests := []struct {
		desc   string
		path   string
		params map[string]interface{}
		status int
	}{
		{"Valid Request", "path", map[string]interface{}{"key": "value"}, http.StatusOK},
		{"Request with Empty Path", "", map[string]interface{}{"key": "value"}, http.StatusOK},
		{"Request With Params", "path", map[string]interface{}{"name": []string{"gofr"}}, http.StatusOK},
	}

	for i, tc := range tests {
		t.Run(tc.desc, func(t *testing.T) {
			ctx := &gofr.Context{Context: context.Background()}
			resp, err := service.Get(ctx, tc.path, tc.params)

			assert.NoError(t, err)
			assert.NotNil(t, resp, "TEST[%d], Failed.\n%s", i, tc.desc)

			defer resp.Body.Close()
		})
	}
}

func TestHTTPService_createAndSendRequest(t *testing.T) {
	// Setup a test server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		//read request body
		var body []byte
		r.Body.Read(body)

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
	}

	ctx := &gofr.Context{Context: context.Background()}
	// when params value is of type []string then last value is sent in request
	resp, err := service.createAndSendRequest(ctx,
		http.MethodPost, "test-path", map[string]interface{}{"key": "value", "name": []string{"gofr", "test"}},
		[]byte("{Test Body}"), map[string]string{"header1": "value1"})

	assert.NoError(t, err)
	assert.NotNil(t, resp, "TEST[%d], Failed.\n%s")

	defer resp.Body.Close()

}
