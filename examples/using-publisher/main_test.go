package main

import (
	"bytes"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestExamplePublisher(t *testing.T) {
	const host = "http://localhost:8100"
	go main()
	time.Sleep(time.Second * 1)

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
	t.Setenv("HTTP_PORT", "8200")

	const host = "http://localhost:8200"
	go main()
	time.Sleep(time.Second * 1)

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
