package service

import (
	"context"
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
			ctx := context.Background()
			resp, err := service.Get(ctx, tc.path, tc.params)

			assert.NoError(t, err)
			assert.NotNil(t, resp, "TEST[%d], Failed.\n%s", i, tc.desc)

			defer resp.Body.Close()
		})
	}
}
