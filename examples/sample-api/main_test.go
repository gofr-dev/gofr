package main

import (
	"bytes"
	"net/http"
	"testing"
	"time"

	"gofr.dev/pkg/gofr/request"
)

func TestIntegration(t *testing.T) {
	go main()
	time.Sleep(3 * time.Second)

	tests := []struct {
		desc       string
		method     string
		endpoint   string
		statusCode int
		body       []byte
	}{
		{"get hello-world", http.MethodGet, "hello-world", http.StatusOK, nil},
		{"get hello", http.MethodGet, "hello?name=random", http.StatusOK, nil},
		{"get json", http.MethodGet, "json", http.StatusOK, nil},
		{"get error", http.MethodGet, "error", http.StatusInternalServerError, nil},
		{"get swagger", http.MethodGet, "/.well-known/swagger", http.StatusOK, nil},
		{"unregistered update route", http.MethodPut, "swagger", http.StatusMethodNotAllowed, []byte(`{}`)},
	}

	for i, tc := range tests {
		req, _ := request.NewMock(tc.method, "http://localhost:4500/"+tc.endpoint, bytes.NewBuffer(tc.body))

		c := http.Client{}

		resp, err := c.Do(req)
		if err != nil {
			t.Errorf("TEST[%v] Failed.\tHTTP request encountered Err: %v\n%s", i, err, tc.desc)
			continue
		}

		if resp.StatusCode != tc.statusCode {
			t.Errorf("TEST[%v] Failed.\tExpected %v\tGot %v\n%s", i, tc.statusCode, resp.StatusCode, tc.desc)
		}

		_ = resp.Body.Close()
	}
}
