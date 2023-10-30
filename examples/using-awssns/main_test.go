package main

import (
	"bytes"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"gofr.dev/pkg/gofr/request"
)

func TestIntegration(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping testing in short mode")
	}

	go main()
	time.Sleep(3 * time.Second)

	tests := []struct {
		method             string
		endpoint           string
		expectedStatusCode int
		body               []byte
	}{
		{http.MethodPost, "publish", http.StatusCreated, []byte(`{"name": "GOFR", "message":  "hi"}`)},
		{http.MethodGet, "subscribe", http.StatusOK, nil},
	}

	for i, tc := range tests {
		req, _ := request.NewMock(tc.method, "http://localhost:8080/"+tc.endpoint, bytes.NewReader(tc.body))
		c := http.Client{}

		resp, err := c.Do(req)
		if err != nil {
			t.Errorf("TEST %v: error while making request err, %v", i+1, err)
			continue
		}

		assert.Equal(t, tc.expectedStatusCode, resp.StatusCode, "Test %v: Failed.\n", i+1)

		_ = resp.Body.Close()
	}
}
