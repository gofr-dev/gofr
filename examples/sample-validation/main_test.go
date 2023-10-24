package main

import (
	"bytes"
	"net/http"
	"testing"
	"time"

	"gofr.dev/pkg/gofr/request"
)

func TestServerValidation(t *testing.T) {
	go main()
	time.Sleep(3 * time.Second)

	tests := []struct {
		desc       string
		method     string
		endpoint   string
		statusCode int
		body       []byte
	}{
		{"create success case", http.MethodPost, "phone", http.StatusCreated, []byte(`{"phone":"+912123456789098", "email": "c.r@yahoo.com"}`)},
		{"create fail case", http.MethodPost, "phone", http.StatusInternalServerError, nil},
		{"invalid endpoint", http.MethodPost, "phone2", http.StatusNotFound, nil},
		{"invalid route", http.MethodGet, "phone", http.StatusMethodNotAllowed, nil},
	}

	for i, tc := range tests {
		req, _ := request.NewMock(tc.method, "http://localhost:8100/"+tc.endpoint, bytes.NewBuffer(tc.body))
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
