package main

import (
	"bytes"
	"net/http"
	"testing"
	"time"

	"gofr.dev/pkg/gofr/request"
)

func TestServerRun(t *testing.T) {
	go main()
	time.Sleep(3 * time.Second)

	tests := []struct {
		desc       string
		method     string
		endpoint   string
		statusCode int
		body       []byte
	}{
		{"get home route", http.MethodGet, "http://localhost:9101", http.StatusSwitchingProtocols, nil},
		{"invalid route", http.MethodPost, "http://localhost:9101/ws", http.StatusMethodNotAllowed, nil},
		{"get ws success", http.MethodGet, "http://localhost:9101/ws", http.StatusSwitchingProtocols, nil},
	}

	for i, tc := range tests {
		req, _ := request.NewMock(tc.method, tc.endpoint, bytes.NewBuffer(tc.body))
		req.Header.Set("Connection", "upgrade")
		req.Header.Set("Upgrade", "websocket")
		req.Header.Set("Sec-Websocket-Version", "13")
		req.Header.Set("Sec-WebSocket-Key", "wehkjeh21-sdjk210-wsknb")

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
