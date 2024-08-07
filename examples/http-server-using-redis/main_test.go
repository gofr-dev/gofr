package main

import (
	"bytes"
	"context"
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
	gofrHTTP "gofr.dev/pkg/gofr/http"
	"gofr.dev/pkg/gofr/logging"
	"gofr.dev/pkg/gofr/testutil"
)

func TestHTTPServerUsingRedis(t *testing.T) {
	const host = "http://localhost:8000"
	go main()
	time.Sleep(time.Second * 1) // Giving some time to start the server

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
		req, _ := http.NewRequest(tc.method, host+tc.path, bytes.NewBuffer(tc.body))
		req.Header.Set("content-type", "application/json")
		c := http.Client{}
		resp, err := c.Do(req)

		require.NoError(t, err, "TEST[%d], Failed.\n%s", i, tc.desc)

		assert.Equal(t, tc.statusCode, resp.StatusCode, "TEST[%d], Failed.\n%s", i, tc.desc)
	}
}

func TestRedisSetHandler(t *testing.T) {
	a := gofr.New()
	logger := logging.NewLogger(logging.DEBUG)
	redisClient, mock := redismock.NewClientMock()

	rc := redis.NewClient(config.NewMockConfig(map[string]string{"REDIS_HOST": "localhost", "REDIS_PORT": "2001"}), logger, a.Metrics())
	rc.Client = redisClient

	mock.ExpectSet("key", "value", 5*time.Minute).SetErr(testutil.CustomError{ErrorMessage: "redis get error"})

	req, _ := http.NewRequestWithContext(context.Background(), http.MethodGet, "http://localhost:5000/handle", bytes.NewBuffer([]byte(`{"key":"value"}`)))
	req.Header.Set("content-type", "application/json")
	gofrReq := gofrHTTP.NewRequest(req)

	ctx := &gofr.Context{Context: context.Background(),
		Request: gofrReq, Container: &container.Container{Logger: logger, Redis: rc}}

	resp, err := RedisSetHandler(ctx)

	assert.Nil(t, resp)
	require.Error(t, err)
}

func TestRedisPipelineHandler(t *testing.T) {
	a := gofr.New()
	logger := logging.NewLogger(logging.DEBUG)
	redisClient, mock := redismock.NewClientMock()

	rc := redis.NewClient(config.NewMockConfig(map[string]string{"REDIS_HOST": "localhost", "REDIS_PORT": "2001"}), logger, a.Metrics())
	rc.Client = redisClient

	mock.ExpectSet("testKey1", "testValue1", time.Minute*5).SetErr(testutil.CustomError{ErrorMessage: "redis get error"})
	mock.ClearExpect()

	req, _ := http.NewRequestWithContext(context.Background(), http.MethodGet, "http://localhost:5000/handle", bytes.NewBuffer([]byte(`{"key":"value"}`)))
	req.Header.Set("content-type", "application/json")

	gofrReq := gofrHTTP.NewRequest(req)

	ctx := &gofr.Context{Context: context.Background(),
		Request: gofrReq, Container: &container.Container{Logger: logger, Redis: rc}}

	resp, err := RedisPipelineHandler(ctx)

	assert.Nil(t, resp)
	require.Error(t, err)
}
