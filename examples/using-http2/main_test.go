package main

import (
	"crypto/tls"
	"net/http"
	"testing"
	"time"

	"gofr.dev/pkg/gofr/request"
)

func TestServerRun(t *testing.T) {
	go main()
	time.Sleep(3 * time.Second)

	//nolint:gosec // TLS InsecureSkipVerify set true.
	tr := &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
	}

	tcs := []struct {
		desc       string
		endpoint   string
		statusCode int
	}{
		{"get success", "/home", http.StatusOK},
		{"get unknown endpoint", "/unknown", http.StatusNotFound},
	}
	for i, tc := range tcs {
		req, _ := request.NewMock(http.MethodGet, "https://localhost:1449"+tc.endpoint, nil)
		c := http.Client{Transport: tr}

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
