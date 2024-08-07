package main

import (
	"bytes"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestIntegration_AddRESTHandlers(t *testing.T) {
	const host = "http://localhost:9090"
	go main()
	time.Sleep(time.Second * 1) // Giving some time to start the server

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
		req, _ := http.NewRequest(tc.method, host+tc.path, bytes.NewReader(tc.body))
		req.Header.Set("content-type", "application/json")

		c := http.Client{}
		resp, err := c.Do(req)

		require.NoError(t, err, "TEST[%d], Failed.\n%s", i, tc.desc)

		assert.Equal(t, tc.statusCode, resp.StatusCode, "TEST[%d], Failed.\n%s", i, tc.desc)
	}
}
