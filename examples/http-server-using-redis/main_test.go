package main

import (
	"bytes"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
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
			http.StatusOK},
		{"post invalid body", http.MethodPost, []byte(`{key:abc}`), "/redis",
			http.StatusInternalServerError},
		{"get handler", http.MethodGet, nil, "/redis/key1", http.StatusOK},
		{"get handler invalid key", http.MethodGet, nil, "/redis/key2",
			http.StatusInternalServerError},
		{"pipeline handler", http.MethodGet, nil, "/redis-pipeline", http.StatusOK},
	}

	for i, tc := range tests {
		req, _ := http.NewRequest(tc.method, host+tc.path, bytes.NewBuffer(tc.body))
		c := http.Client{}
		resp, err := c.Do(req)

		assert.Nil(t, err, "TEST[%d], Failed.\n%s", i, tc.desc)

		assert.Equal(t, tc.statusCode, resp.StatusCode, "TEST[%d], Failed.\n%s", i, tc.desc)
	}
}
