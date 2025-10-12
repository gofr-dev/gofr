package main

import (
	"bytes"
	"context"
	"fmt"
	"net/http"
	"os"
	"testing"
	"time"

	"github.com/go-redis/redismock/v9"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"gofr.dev/pkg/gofr"
	"gofr.dev/pkg/gofr/config"
	"gofr.dev/pkg/gofr/container"
	"gofr.dev/pkg/gofr/datasource/redis"
	gofrHTTP "gofr.dev/pkg/gofr/http"
	"gofr.dev/pkg/gofr/logging"
	"gofr.dev/pkg/gofr/testutil"
)

func TestMain(m *testing.M) {
	os.Setenv("GOFR_TELEMETRY", "false")
	m.Run()
}

func TestHTTPServerUsingRedis(t *testing.T) {
	configs := testutil.NewServerConfigs(t)

	go main()
	time.Sleep(100 * time.Millisecond) // Giving some time to start the server

	tests := []struct {
		desc       string
		method     string
		body       []byte
		path       string
		statusCode int
	}{
		{"post handler", http.MethodPost, []byte(`{"key1":"GoFr"}`), "/redis",
			http.StatusCreated},
		{"post invalid body", http.MethodPost, []byte(`{key:abc}`), "/redis",
			http.StatusInternalServerError},
		{"get handler", http.MethodGet, nil, "/redis/key1", http.StatusOK},
		{"get handler invalid key", http.MethodGet, nil, "/redis/key2",
			http.StatusInternalServerError},
		{"pipeline handler", http.MethodGet, nil, "/redis-pipeline", http.StatusOK},
	}

	for i, tc := range tests {
		req, _ := http.NewRequest(tc.method, configs.HTTPHost+tc.path, bytes.NewBuffer(tc.body))
		req.Header.Set("content-type", "application/json")
		c := http.Client{}
		resp, err := c.Do(req)

		require.NoError(t, err, "TEST[%d], Failed.\n%s", i, tc.desc)

		assert.Equal(t, tc.statusCode, resp.StatusCode, "TEST[%d], Failed.\n%s", i, tc.desc)
	}
}

func TestRedisSetHandler(t *testing.T) {
	configs := testutil.NewServerConfigs(t)

	a := gofr.New()
	logger := logging.NewLogger(logging.DEBUG)
	redisClient, mock := redismock.NewClientMock()

	rc := redis.NewClient(config.NewMockConfig(map[string]string{"REDIS_HOST": "localhost", "REDIS_PORT": "2001"}), logger, a.Metrics())
	rc.Client = redisClient

	mock.ExpectSet("key", "value", 5*time.Minute).SetErr(testutil.CustomError{ErrorMessage: "redis get error"})

	req, _ := http.NewRequestWithContext(context.Background(), http.MethodGet, fmt.Sprintf("http://localhost:%d/handle", configs.HTTPPort), bytes.NewBuffer([]byte(`{"key":"value"}`)))
	req.Header.Set("content-type", "application/json")
	gofrReq := gofrHTTP.NewRequest(req)

	ctx := &gofr.Context{Context: context.Background(),
		Request: gofrReq, Container: &container.Container{Logger: logger, Redis: rc}}

	resp, err := RedisSetHandler(ctx)

	assert.Nil(t, resp)
	require.Error(t, err)
}

func TestRedisGetHandler(t *testing.T) {
	configs := testutil.NewServerConfigs(t)

	a := gofr.New()
	logger := logging.NewLogger(logging.DEBUG)
	redisClient, mock := redismock.NewClientMock()

	rc := redis.NewClient(config.NewMockConfig(map[string]string{"REDIS_HOST": "localhost", "REDIS_PORT": "2001"}), logger, a.Metrics())
	rc.Client = redisClient

	mock.ExpectSet("testKey1", "testValue1", time.Minute*5).SetErr(testutil.CustomError{ErrorMessage: "redis get error"})
	mock.ClearExpect()

	mock.ExpectGet("testKey1").SetErr(testutil.CustomError{ErrorMessage: "redis get error"})
	mock.ClearExpect()

	req, _ := http.NewRequestWithContext(context.Background(), http.MethodGet, fmt.Sprintf("http://localhost:%d/handle", configs.HTTPPort), bytes.NewBuffer([]byte(`{"key":"value"}`)))
	req.Header.Set("content-type", "application/json")
	gofrReq := gofrHTTP.NewRequest(req)

	ctx := &gofr.Context{Context: context.Background(),
		Request: gofrReq, Container: &container.Container{Logger: logger, Redis: rc}}

	resp, err := RedisGetHandler(ctx)

	assert.Nil(t, resp)
	require.Error(t, err)
}

func TestWarmupCache_Success(t *testing.T) {
	logger := logging.NewLogger(logging.DEBUG)
	redisClient, mock := redismock.NewClientMock()
	mock.ExpectSet("initial-data", "This is some data cached at startup.", 0).SetVal("OK")

	rc := &redis.Redis{Client: redisClient}

	ctx := &gofr.Context{Context: context.Background(), Container: &container.Container{Logger: logger, Redis: rc}}

	err := warmupCache(ctx)
	require.NoError(t, err)
}

func TestWarmupCache_Failure(t *testing.T) {

	logger := logging.NewLogger(logging.DEBUG)
	redisClient, mock := redismock.NewClientMock()
	mock.ExpectSet("initial-data", "This is some data cached at startup.", 0).
		SetErr(fmt.Errorf("mocked redis error"))

	rc := &redis.Redis{Client: redisClient}

	rc.Client = redisClient

	ctx := &gofr.Context{Context: context.Background(), Container: &container.Container{Logger: logger, Redis: rc}}

	err := warmupCache(ctx)
	require.Error(t, err)
}

func TestRedisPipelineHandler(t *testing.T) {
	configs := testutil.NewServerConfigs(t)

	a := gofr.New()
	logger := logging.NewLogger(logging.DEBUG)
	redisClient, mock := redismock.NewClientMock()

	rc := redis.NewClient(config.NewMockConfig(map[string]string{"REDIS_HOST": "localhost", "REDIS_PORT": "2001"}), logger, a.Metrics())
	rc.Client = redisClient

	mock.ExpectSet("testKey1", "testValue1", time.Minute*5).SetErr(testutil.CustomError{ErrorMessage: "redis get error"})
	mock.ClearExpect()

	req, _ := http.NewRequestWithContext(context.Background(), http.MethodGet, fmt.Sprint("http://localhost:", configs.HTTPHost, "/handle"), bytes.NewBuffer([]byte(`{"key":"value"}`)))
	req.Header.Set("content-type", "application/json")

	gofrReq := gofrHTTP.NewRequest(req)

	ctx := &gofr.Context{Context: context.Background(),
		Request: gofrReq, Container: &container.Container{Logger: logger, Redis: rc}}

	resp, err := RedisPipelineHandler(ctx)

	assert.Nil(t, resp)
	require.Error(t, err)
}
