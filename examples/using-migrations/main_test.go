package main

import (
	"bytes"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestExampleMigration(t *testing.T) {
	const host = "http://localhost:9000"
	go main()
	time.Sleep(time.Second * 3) // Giving some time to start the server

	tests := []struct {
		desc       string
		method     string
		path       string
		body       []byte
		statusCode int
	}{
		{"post new employee with valid data", http.MethodPost, "/employee",
			[]byte(`{"id":2,"name":"John","gender":"Male","contact_number":1234567890,"dob":"2000-01-01"}`), 200},
		{"get employee with valid name", http.MethodGet, "/employee?name=John", nil, 200},
		{"get employee with empty name", http.MethodGet, "/employee", nil, http.StatusInternalServerError},
		{"post new employee with invalid data", http.MethodPost, "/employee", []byte(`{"id":2"}`),
			http.StatusInternalServerError},
	}

	for i, tc := range tests {
		req, _ := http.NewRequest(tc.method, host+tc.path, bytes.NewBuffer(tc.body))
		c := http.Client{}
		resp, err := c.Do(req)

		assert.Nil(t, err, "TEST[%d], Failed.\n%s", i, tc.desc)

		assert.Equal(t, tc.statusCode, resp.StatusCode, "TEST[%d], Failed.\n%s", i, tc.desc)
	}
}
