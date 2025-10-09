package main

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/go-redis/redismock/v9"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"gofr.dev/pkg/gofr"
	"gofr.dev/pkg/gofr/config"
	"gofr.dev/pkg/gofr/container"
	"gofr.dev/pkg/gofr/datasource/redis"
	"gofr.dev/pkg/gofr/logging"
	"gofr.dev/pkg/gofr/service"
	"gofr.dev/pkg/gofr/testutil"
)

func TestMain(m *testing.M) {
	os.Setenv("GOFR_TELEMETRY", "false")
	m.Run()
}

func TestIntegration_SimpleAPIServer(t *testing.T) {
	host := "http://localhost:9000"

	port := testutil.GetFreePort(t)
	t.Setenv("METRICS_PORT", strconv.Itoa(port))

	go main()
	time.Sleep(100 * time.Millisecond) // Giving some time to start the server

	tests := []struct {
		desc string
		path string
		body any
	}{
		{"hello handler", "/hello", "Hello World!"},
		{"hello handler with query parameter", "/hello?name=gofr", "Hello gofr!"},
		{"redis handler", "/redis", ""},
		{"trace handler", "/trace", ""},
		{"mysql handler", "/mysql", float64(4)},
	}

	for i, tc := range tests {
		req, _ := http.NewRequest(http.MethodGet, host+tc.path, nil)
		req.Header.Set("content-type", "application/json")

		c := http.Client{}
		resp, err := c.Do(req)

		var data = struct {
			Data any `json:"data"`
		}{}

		b, err := io.ReadAll(resp.Body)

		require.NoError(t, err, "TEST[%d], Failed.\n%s", i, tc.desc)

		_ = json.Unmarshal(b, &data)

		assert.Equal(t, tc.body, data.Data, "TEST[%d], Failed.\n%s", i, tc.desc)

		require.NoError(t, err, "TEST[%d], Failed.\n%s", i, tc.desc)

		assert.Equal(t, http.StatusOK, resp.StatusCode, "TEST[%d], Failed.\n%s", i, tc.desc)

		resp.Body.Close()
	}
}

func TestIntegration_SimpleAPIServer_Errors(t *testing.T) {
	host := "http://localhost:9000"

	tests := []struct {
		desc       string
		path       string
		body       any
		statusCode int
	}{
		{
			desc:       "error handler called",
			path:       "/error",
			statusCode: http.StatusInternalServerError,
			body:       map[string]any{"message": "some error occurred"},
		},
		{
			desc:       "empty route",
			path:       "/",
			statusCode: http.StatusNotFound,
			body:       map[string]any{"message": "route not registered"},
		},
		{
			desc:       "route not registered with the server",
			path:       "/route",
			statusCode: http.StatusNotFound,
			body:       map[string]any{"message": "route not registered"},
		},
	}

	for i, tc := range tests {
		req, _ := http.NewRequest(http.MethodGet, host+tc.path, nil)
		req.Header.Set("content-type", "application/json")

		c := http.Client{}
		resp, err := c.Do(req)

		var data = struct {
			Error any `json:"error"`
		}{}

		b, err := io.ReadAll(resp.Body)

		require.NoError(t, err, "TEST[%d], Failed.\n%s", i, tc.desc)

		_ = json.Unmarshal(b, &data)

		assert.Equal(t, tc.body, data.Error, "TEST[%d], Failed.\n%s", i, tc.desc)

		require.NoError(t, err, "TEST[%d], Failed.\n%s", i, tc.desc)

		assert.Equal(t, tc.statusCode, resp.StatusCode, "TEST[%d], Failed.\n%s", i, tc.desc)

		resp.Body.Close()
	}
}

func TestIntegration_SimpleAPIServer_Health(t *testing.T) {
	host := "http://localhost:9000"

	tests := []struct {
		desc       string
		path       string
		statusCode int
	}{
		{"health handler", "/.well-known/health", http.StatusOK}, // Health check should be added by the framework.
		{"favicon handler", "/favicon.ico", http.StatusOK},       // Favicon should be added by the framework.
	}

	for i, tc := range tests {
		req, _ := http.NewRequest(http.MethodGet, host+tc.path, nil)
		req.Header.Set("content-type", "application/json")

		c := http.Client{}
		resp, err := c.Do(req)

		require.NoError(t, err, "TEST[%d], Failed.\n%s", i, tc.desc)

		assert.Equal(t, tc.statusCode, resp.StatusCode, "TEST[%d], Failed.\n%s", i, tc.desc)
	}
}

func TestRedisHandler(t *testing.T) {
	metricsPort := testutil.GetFreePort(t)
	httpPort := testutil.GetFreePort(t)

	t.Setenv("METRICS_PORT", strconv.Itoa(metricsPort))
	t.Setenv("HTTP_PORT", strconv.Itoa(httpPort))

	a := gofr.New()
	logger := logging.NewLogger(logging.DEBUG)
	redisClient, mock := redismock.NewClientMock()

	rc := redis.NewClient(config.NewMockConfig(map[string]string{"REDIS_HOST": "localhost", "REDIS_PORT": "2001"}), logger, a.Metrics())
	rc.Client = redisClient

	mock.ExpectGet("test").SetErr(testutil.CustomError{ErrorMessage: "redis get error"})

	ctx := &gofr.Context{Context: context.Background(),
		Request: nil, Container: &container.Container{Logger: logger, Redis: rc}}

	resp, err := RedisHandler(ctx)

	assert.Nil(t, resp)
	require.Error(t, err)
}

// MockRequest implements the Request interface for testing
type MockRequest struct {
	*http.Request
	params map[string]string
}

func (m *MockRequest) HostName() string {
	if m.Request != nil {
		return m.Request.Host
	}

	return ""
}

func (m *MockRequest) Params(s string) []string {
	if m.Request != nil {
		return m.Request.URL.Query()[s]
	}

	return nil
}

func NewMockRequest(req *http.Request) *MockRequest {
	// Parse query parameters
	queryParams := make(map[string]string)
	for k, v := range req.URL.Query() {
		if len(v) > 0 {
			queryParams[k] = v[0]
		}
	}

	return &MockRequest{
		Request: req,
		params:  queryParams,
	}
}

// Param returns URL query parameters
func (m *MockRequest) Param(key string) string {
	return m.params[key]
}

// PathParam returns URL path parameters
func (m *MockRequest) PathParam(key string) string {
	return ""
}

// Bind implements the Bind method required by the Request interface
func (m *MockRequest) Bind(i any) error {
	return nil
}

// createTestContext sets up a GoFr context for unit tests with a given URL and optional mock container.
func createTestContext(method, url string, mockContainer *container.Container) *gofr.Context {
	req := httptest.NewRequest(method, url, nil)
	mockReq := NewMockRequest(req)

	var c *container.Container
	if mockContainer != nil {
		c = mockContainer
	} else {
		c = &container.Container{Logger: logging.NewLogger(logging.DEBUG)}
	}

	logger := c.Logger

	return &gofr.Context{
		Context:       req.Context(),
		Request:       mockReq,
		Container:     c,
		ContextLogger: *logging.NewContextLogger(req.Context(), logger),
	}
}

func TestHelloHandler(t *testing.T) {
	// With name parameter
	ctx := createTestContext(http.MethodGet, "/hello?name=test", nil)
	resp, err := HelloHandler(ctx)
	assert.NoError(t, err)
	assert.Equal(t, "Hello test!", resp)

	// Without name parameter
	ctx = createTestContext(http.MethodGet, "/hello", nil)
	resp, err = HelloHandler(ctx)
	assert.NoError(t, err)
	assert.Equal(t, "Hello World!", resp)
}

func TestErrorHandler(t *testing.T) {
	ctx := createTestContext(http.MethodGet, "/error", nil)

	resp, err := ErrorHandler(ctx)
	assert.Nil(t, resp)
	assert.Error(t, err)
	assert.Equal(t, "some error occurred", err.Error())
}

func TestMysqlHandler(t *testing.T) {
	mockContainer, mocks := container.NewMockContainer(t)

	// Setup SQL mock to return 4
	mocks.SQL.ExpectQuery("select 2+2").
		WillReturnRows(mocks.SQL.NewRows([]string{"value"}).AddRow(4))

	ctx := createTestContext(http.MethodGet, "/mysql", mockContainer)

	resp, err := MysqlHandler(ctx)
	assert.NoError(t, err)
	assert.Equal(t, 4, resp)
}

func TestTraceHandler(t *testing.T) {
	mockContainer, mocks := container.NewMockContainer(t, container.WithMockHTTPService())

	// Redis expectations
	mocks.Redis.EXPECT().Ping(gomock.Any()).Return(nil).Times(5)

	// HTTP service mock
	httpService := mocks.HTTPService
	mockResp := &http.Response{
		StatusCode: http.StatusOK,
		Body:       io.NopCloser(strings.NewReader(`{"data":"mock data"}`)),
	}
	httpService.EXPECT().Get(gomock.Any(), "redis", gomock.Any()).Return(mockResp, nil)

	// Attach service to container
	mockContainer.Services = map[string]service.HTTP{
		"anotherService": httpService,
	}

	ctx := createTestContext(http.MethodGet, "/trace", mockContainer)

	resp, err := TraceHandler(ctx)
	assert.NoError(t, err)
	assert.Equal(t, "mock data", resp)
}
