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
	time.Sleep(time.Second * 5)

	tcs := []struct {
		desc       string
		method     string
		endpoint   string
		statusCode int
		body       []byte
	}{
		{"call to get key", http.MethodGet, "config/key123", http.StatusInternalServerError, nil},
		{"call to create key", http.MethodPost, "config", http.StatusCreated, []byte(`{}`)},
		{"call to delete a non existent key", http.MethodDelete, "config/key123", http.StatusNoContent, nil},
	}

	for i, tc := range tcs {
		req, _ := request.NewMock(tc.method, "http://localhost:9098/"+tc.endpoint, bytes.NewBuffer(tc.body))
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
