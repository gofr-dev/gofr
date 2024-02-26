package main

import (
	"bytes"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
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
			expectedStatusCode: http.StatusOK,
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
			expectedStatusCode: http.StatusOK,
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
		resp, err := c.Do(req)

		assert.Equal(t, tc.expectedStatusCode, resp.StatusCode, "TEST[%d], Failed.\n%s", i, tc.desc)
		assert.NoError(t, err, "TEST[%d], Failed.\n%s", i, tc.desc)

		defer resp.Body.Close()

	}
}
