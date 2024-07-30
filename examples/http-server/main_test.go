package main

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"testing"
	"time"

	"github.com/go-redis/redismock/v9"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"gofr.dev/pkg/gofr"
	"gofr.dev/pkg/gofr/config"
	"gofr.dev/pkg/gofr/container"
	"gofr.dev/pkg/gofr/datasource/redis"
	"gofr.dev/pkg/gofr/logging"
	"gofr.dev/pkg/gofr/testutil"
)

const host = "http://localhost:9000"

func TestIntegration_SimpleAPIServer(t *testing.T) {
	go main()
	time.Sleep(time.Second * 3) // Giving some time to start the server

	tests := []struct {
		desc string
		path string
		body interface{}
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
			Data interface{} `json:"data"`
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
	tests := []struct {
		desc       string
		path       string
		body       interface{}
		statusCode int
	}{
		{
			desc:       "error handler called",
			path:       "/error",
			statusCode: http.StatusInternalServerError,
			body:       map[string]interface{}{"message": "some error occurred"},
		},
		{
			desc:       "empty route",
			path:       "/",
			statusCode: http.StatusNotFound,
			body:       map[string]interface{}{"message": "route not registered"},
		},
		{
			desc:       "route not registered with the server",
			path:       "/route",
			statusCode: http.StatusNotFound,
			body:       map[string]interface{}{"message": "route not registered"},
		},
	}

	for i, tc := range tests {
		req, _ := http.NewRequest(http.MethodGet, host+tc.path, nil)
		req.Header.Set("content-type", "application/json")

		c := http.Client{}
		resp, err := c.Do(req)

		var data = struct {
			Error interface{} `json:"error"`
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
	tests := []struct {
		desc       string
		path       string
		statusCode int
	}{
		{"health handler", "/.well-known/health", http.StatusOK}, // Health check should be added by the framework.
		{"favicon handler", "/favicon.ico", http.StatusOK},       //Favicon should be added by the framework.
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
