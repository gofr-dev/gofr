package main

import (
	"bytes"
	"fmt"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"gofr.dev/pkg/gofr/testutil"
)

func TestExamplePublisher(t *testing.T) {
	configs := testutil.NewServerConfigs(t)

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
		req, _ := http.NewRequest(http.MethodPost, configs.HTTPHost+tc.path, bytes.NewBuffer(tc.body))
		req.Header.Set("content-type", "application/json")
		resp, err := c.Do(req)
		defer resp.Body.Close()

		assert.Equal(t, tc.expectedStatusCode, resp.StatusCode, "TEST[%d], Failed.\n%s", i, tc.desc)
		require.NoError(t, err, "TEST[%d], Failed.\n%s", i, tc.desc)
	}
}

func TestExamplePublisherError(t *testing.T) {
	t.Setenv("PUBSUB_BROKER", "localhost:1012")

	configs := testutil.NewServerConfigs(t)

	host := fmt.Sprint("http://localhost:", configs.HTTPPort)

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
