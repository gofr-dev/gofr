package main

import (
	"bytes"
	"fmt"
	"gofr.dev/pkg/gofr/testutil"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestExamplePublisher(t *testing.T) {
	const host = "http://localhost:8100"

	port := testutil.GetFreePort(t)
	t.Setenv("METRICS_PORT", fmt.Sprint(port))

	go main()
	time.Sleep(200 * time.Millisecond)

	testCases := []struct {
		desc               string
		path               string
		body               []byte
		expectedStatusCode int
		expectedError      error
	}{
		{
			desc:               "valid order",
			path:               "/publish-order",
			body:               []byte(`{"data":{"orderId":"123","status":"pending"}}`),
			expectedStatusCode: http.StatusCreated,
		},
		{
			desc:               "invalid order",
			path:               "/publish-order",
			body:               []byte(`{"data":,"status":"pending"}`),
			expectedStatusCode: http.StatusInternalServerError,
		},
		{
			desc:               "valid product",
			path:               "/publish-product",
			body:               []byte(`{"data":{"productId":"123","price":"599"}}`),
			expectedStatusCode: http.StatusCreated,
		},
		{
			desc:               "invalid product",
			path:               "/publish-product",
			body:               []byte(`{"data":,"price":"pending"}`),
			expectedStatusCode: http.StatusInternalServerError,
		},
	}

	c := http.Client{}

	for i, tc := range testCases {
		req, _ := http.NewRequest(http.MethodPost, host+tc.path, bytes.NewBuffer(tc.body))
		req.Header.Set("content-type", "application/json")
		resp, err := c.Do(req)
		defer resp.Body.Close()

		assert.Equal(t, tc.expectedStatusCode, resp.StatusCode, "TEST[%d], Failed.\n%s", i, tc.desc)
		require.NoError(t, err, "TEST[%d], Failed.\n%s", i, tc.desc)
	}
}

func TestExamplePublisherError(t *testing.T) {
	t.Setenv("PUBSUB_BROKER", "localhost:1012")

	httpPort := testutil.GetFreePort(t)
	t.Setenv("HTTP_PORT", fmt.Sprint(httpPort))

	metricsPort := testutil.GetFreePort(t)
	t.Setenv("METRICS_PORT", fmt.Sprint(metricsPort))

	const host = "http://localhost:8200"
	go main()
	time.Sleep(200 * time.Millisecond)

	testCases := []struct {
		desc               string
		path               string
		body               []byte
		expectedStatusCode int
		expectedError      error
	}{
		{
			desc:               "valid order",
			path:               "/publish-order",
			body:               []byte(`{"data":{"orderId":"123","status":"pending"}}`),
			expectedStatusCode: http.StatusInternalServerError,
		},
		{
			desc:               "valid product",
			path:               "/publish-product",
			body:               []byte(`{"data":{"productId":"123","price":"599"}}`),
			expectedStatusCode: http.StatusInternalServerError,
		},
	}

	c := http.Client{}

	for i, tc := range testCases {
		req, _ := http.NewRequest(http.MethodPost, host+tc.path, bytes.NewBuffer(tc.body))
		req.Header.Set("content-type", "application/json")
		resp, err := c.Do(req)

		assert.Equal(t, tc.expectedStatusCode, resp.StatusCode, "TEST[%d], Failed.\n%s", i, tc.desc)
		require.NoError(t, err, "TEST[%d], Failed.\n%s", i, tc.desc)

		defer resp.Body.Close()
	}
}
