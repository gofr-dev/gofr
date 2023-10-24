package main

import (
	"bytes"
	"net/http"
	"testing"
	"time"

	"gofr.dev/pkg/gofr/request"
)

func TestServerIntegration(t *testing.T) {
	go main()
	time.Sleep(10 * time.Second)

	tests := []struct {
		desc        string
		method      string
		endpoint    string
		statusCode  int
		body        []byte
		contentType string
	}{
		{"get test success", http.MethodGet, "test", http.StatusOK, nil, "text/html"},
		{"get invalid route", http.MethodGet, "test2", http.StatusNotFound, nil, "application/json"},
		{"get image success", http.MethodGet, "image", http.StatusOK, nil, "image/png"},
		{"unregistered update route", http.MethodPut, "unknown", http.StatusNotFound, []byte(`{}`), "application/json"},
	}

	for i, tc := range tests {
		req, _ := request.NewMock(tc.method, "http://localhost:8000/"+tc.endpoint, bytes.NewBuffer(tc.body))

		c := http.Client{}

		resp, err := c.Do(req)
		if err != nil {
			t.Errorf("TEST[%v] Failed.\tHTTP request encountered Err: %v\n%s", i, err, tc.desc)
			continue
		}

		if resp.StatusCode != tc.statusCode {
			t.Errorf("TEST[%v] Failed.\tExpected %v\tGot %v\n%s", i, tc.statusCode, resp.StatusCode, tc.desc)
		}

		if resp.Header.Get("Content-type") != tc.contentType {
			t.Errorf("TEST[%v] Failed.\tExpected %v\tGot %v\n%s", i, tc.contentType, resp.Header.Get("Content-type"), tc.desc)
		}

		_ = resp.Body.Close()
	}
}
