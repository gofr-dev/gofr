package service

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"

	"gofr.dev/pkg/gofr/testutil"
)

func TestHTTPService_HealthCheck(t *testing.T) {
	ctrl := gomock.NewController(t)

	metrics := NewMockMetrics(ctrl)

	// Setup a test server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// read request body
		assert.Equal(t, "/.well-known/alive", r.URL.Path)

		w.Write([]byte(`{"data":"UP"}`))
	}))
	defer server.Close()

	service := NewHTTPService(server.URL, testutil.NewMockLogger(testutil.INFOLOG), metrics)

	ctx := context.Background()

	metrics.EXPECT().RecordHistogram(ctx, "app_http_service_response", gomock.Any(), "path", server.URL, "method", http.MethodGet, "status", fmt.Sprintf("%v", http.StatusOK)).AnyTimes()

	// when params value is of type []string then last value is sent in request
	resp := service.HealthCheck(ctx)

	assert.Equal(t, &Health{Status: serviceUp, Details: map[string]interface{}{"host": server.URL[7:]}},
		resp, "TEST[%d], Failed.\n%s")
}

func TestHTTPService_HealthCheckCustomURL(t *testing.T) {
	ctrl := gomock.NewController(t)

	metrics := NewMockMetrics(ctrl)

	// Setup a test server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// read request body
		assert.Equal(t, "/.well-known/ready", r.URL.Path)

		w.Write([]byte(`{"data":"UP"}`))
	}))
	defer server.Close()

	service := NewHTTPService(server.URL, testutil.NewMockLogger(testutil.INFOLOG), metrics,
		&HealthConfig{HealthEndpoint: ".well-known/ready"})

	ctx := context.Background()

	metrics.EXPECT().RecordHistogram(ctx, "app_http_service_response", gomock.Any(), "path", server.URL, "method", http.MethodGet, "status", fmt.Sprintf("%v", http.StatusOK)).AnyTimes()

	// when params value is of type []string then last value is sent in request
	resp := service.HealthCheck(ctx)

	assert.Equal(t, &Health{Status: serviceUp, Details: map[string]interface{}{"host": server.URL[7:]}},
		resp, "TEST[%d], Failed.\n%s")
}
