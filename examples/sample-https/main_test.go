package main

import (
	"bytes"
	"crypto/tls"
	"net/http"
	"testing"
	"time"

	"gofr.dev/pkg/gofr/request"
)

func TestServerRun(t *testing.T) {
	t.Setenv("VALIDATE_HEADERS", "Custom-Header")

	go main()
	time.Sleep(3 * time.Second)

	//nolint:gosec // TLS InsecureSkipVerify set true.
	tr := &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
	}

	tests := []struct {
		desc       string
		method     string
		endpoint   string
		statusCode int
		headers    map[string]string
		body       []byte
	}{
		{"get hello-world success", http.MethodGet, "https://localhost:1443/hello-world", http.StatusOK, nil, nil},
		{"get hello success", http.MethodGet, "https://localhost:1443/hello/", http.StatusOK, nil, nil},
		{"post success", http.MethodPost, "https://localhost:1443/post", http.StatusCreated, nil, []byte(`{"Username":"username"}`)},
		{"post existent entity", http.MethodPost, "https://localhost:1443/post/", http.StatusOK, nil, []byte(`{"Username":"alreadyExist"}`)},
		// http will be redirected to https as redirect is set to true in https configuration
		{"get hello over https redirect", http.MethodGet, "http://localhost:9007/hello?name=random", http.StatusOK, nil, nil},
		{"get multiple errors", http.MethodGet, "http://localhost:9007/multiple-errors", http.StatusInternalServerError, nil, nil},
		{"get multiple errors by id", http.MethodGet, "http://localhost:9007/multiple-errors?id=1", http.StatusBadRequest, nil, nil},
		{"get hearthbeat", http.MethodGet, "http://localhost:9007/.well-known/heartbeat", http.StatusOK,
			map[string]string{"Content-Type": "application/json"}, nil},
		{"get error", http.MethodGet, "http://localhost:9007/error", http.StatusNotFound, nil, nil},
	}

	for i, tc := range tests {
		req, _ := request.NewMock(tc.method, tc.endpoint, bytes.NewBuffer(tc.body))
		c := http.Client{Transport: tr}

		if tc.headers == nil {
			req.Header.Set("Custom-Header", "test")
		}

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
