package main

import (
	"io"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func Test_main(t *testing.T) {
	const host = "http://localhost:9001"
	c := &http.Client{}

	go main()
	time.Sleep(time.Second * 3)

	testCases := []struct {
		desc        string
		path        string
		statusCode  int
		expectedRes string
	}{
		{
			desc:        "simple service handler",
			path:        "/fact",
			expectedRes: `{"data":{"fact":"Cats have 3 eyelids.","length":20}}` + "\n",
			statusCode:  200,
		},
		{
			desc: "health check",
			path: "/.well-known/health",
			expectedRes: `{"data":{"cat-facts":{"status":"UP","details":{"host":"catfact.ninja"}},` +
				`"fact-checker":{"status":"DOWN","details":{"error":"service down","host":"catfact.ninja"}}}}` + "\n",
			statusCode: 200,
		},
	}

	for i, tc := range testCases {
		req, _ := http.NewRequest(http.MethodGet, host+tc.path, nil)
		resp, err := c.Do(req)

		assert.Nil(t, err, "TEST[%d], Failed.\n%s", i, tc.desc)

		bodyBytes, err := io.ReadAll(resp.Body)

		assert.Nil(t, err, "TEST[%d], Failed.\n%s", i, tc.desc)

		assert.Equal(t, tc.expectedRes, string(bodyBytes), "TEST[%d], Failed.\n%s", i, tc.desc)

		assert.Equal(t, tc.statusCode, resp.StatusCode, "TEST[%d], Failed.\n%s", i, tc.desc)

		resp.Body.Close()
	}
}
