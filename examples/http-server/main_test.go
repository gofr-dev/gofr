package main

import (
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestSimpleAPIServer(t *testing.T) {
	const host = "http://localhost:9000"
	go main()
	time.Sleep(time.Second * 3) // Giving some time to start the server

	tests := []struct {
		desc       string
		path       string
		statusCode int
	}{
		{"empty path", "/", 404},
		{"hello handler", "/hello", 200},
		{"hello handler with query parameter", "/hello?name=gofr", 200},
		{"error handler", "/error", 500},
		{"redis handler", "/redis", 200},
		{"trace handler", "/trace", 200},
		{"mysql handler", "/mysql", 200},
		{"favicon handler", "/favicon.ico", 200}, //Favicon should be added by the framework.
	}

	for i, tc := range tests {
		req, _ := http.NewRequest("GET", host+tc.path, nil)
		c := http.Client{}
		resp, err := c.Do(req)

		assert.Nil(t, err, "TEST[%d], Failed.\n%s", i, tc.desc)

		assert.Equal(t, tc.statusCode, resp.StatusCode, "TEST[%d], Failed.\n%s", i, tc.desc)
	}
}
