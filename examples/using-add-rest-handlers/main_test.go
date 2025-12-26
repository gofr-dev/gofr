package main

import (
	"bytes"
	"net/http"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"gofr.dev/pkg/gofr/testutil"
)

func TestMain(m *testing.M) {
	os.Setenv("GOFR_TELEMETRY", "false")
	m.Run()
}

func TestIntegration_AddRESTHandlers(t *testing.T) {
	configs := testutil.NewServerConfigs(t)

	go main()

	// Wait for server to be ready with retry logic
	time.Sleep(500 * time.Millisecond)

	// Verify server is ready before running tests
	c := &http.Client{Timeout: 2 * time.Second}
	for i := 0; i < 10; i++ {
		resp, err := c.Get(configs.HTTPHost + "/.well-known/health")
		if err == nil {
			resp.Body.Close()
			break
		}
		if i == 9 {
			t.Fatalf("Server failed to start: %v", err)
		}
		time.Sleep(100 * time.Millisecond)
	}

	tests := []struct {
		desc       string
		method     string
		path       string
		body       []byte
		statusCode int
	}{
		{"empty path", http.MethodGet, "/", nil, 404},
		{"success Create", http.MethodPost, "/user",
			[]byte(`{"id":10, "name":"john doe", "age":99, "isEmployed": true}`), 201},
		{"success GetAll", http.MethodGet, "/user", nil, 200},
		{"success Get", http.MethodGet, "/user/10", nil, 200},
		{"success Update", http.MethodPut, "/user/10",
			[]byte(`{"name":"john doe", "age":99, "isEmployed": false}`), 200},
		{"success Delete", http.MethodDelete, "/user/10", nil, 204},
	}

	for i, tc := range tests {
		req, _ := http.NewRequest(tc.method, configs.HTTPHost+tc.path, bytes.NewReader(tc.body))
		req.Header.Set("content-type", "application/json")

		resp, err := c.Do(req)

		require.NoError(t, err, "TEST[%d], Failed.\n%s", i, tc.desc)

		assert.Equal(t, tc.statusCode, resp.StatusCode, "TEST[%d], Failed.\n%s", i, tc.desc)

		resp.Body.Close()
	}
}
