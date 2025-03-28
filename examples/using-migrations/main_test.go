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

func TestExampleMigration(t *testing.T) {
	configs := testutil.NewServerConfigs(t)

	go main()
	time.Sleep(100 * time.Millisecond) // Giving some time to start the server

	tests := []struct {
		desc       string
		method     string
		path       string
		body       []byte
		statusCode int
	}{
		{"post new employee with valid data", http.MethodPost, "/employee",
			[]byte(`{"id":2,"name":"John","gender":"Male","contact_number":1234567890,"dob":"2000-01-01"}`), 201},
		{"get employee with valid name", http.MethodGet, "/employee?name=John", nil, 200},
		{"get employee does not exist", http.MethodGet, "/employee?name=Invalid", nil, 500},
		{"get employee with empty name", http.MethodGet, "/employee", nil, http.StatusInternalServerError},
		{"post new employee with invalid data", http.MethodPost, "/employee", []byte(`{"id":2"}`),
			http.StatusInternalServerError},
		{"post new employee with invalid gender", http.MethodPost, "/employee",
			[]byte(`{"id":2,"name":"John","gender":"Male123","contact_number":1234567890,"dob":"2000-01-01"}`), 500},
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
