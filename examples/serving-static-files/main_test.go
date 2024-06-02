package main

import (
	"bytes"
	"io"
	"net/http"
	"strconv"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestFileServer(t *testing.T) {
	const host = "http://localhost:9000"
	go main()
	time.Sleep(time.Second * 3)
	tests := []struct {
		desc                       string
		method                     string
		path                       string
		body                       []byte
		statusCode                 int
		expectedBody               string
		expectedBodyLength         int
		expectedResponseHeaderType string
	}{
		{
			desc:         "check static files",
			method:       http.MethodGet,
			path:         "/public/",
			statusCode:   http.StatusOK,
			expectedBody: "<pre>\n<a href=\"hardhat.jpeg\">hardhat.jpeg</a>\n<a href=\"industrial.jpeg\">industrial.jpeg</a>\n<a href=\"skross.png\">skross.png</a>\n</pre>\n",
		},
		{
			desc:                       "check file content hardhat.jpeg",
			method:                     http.MethodGet,
			path:                       "/public/hardhat.jpeg",
			statusCode:                 http.StatusOK,
			expectedBodyLength:         14779,
			expectedResponseHeaderType: "image/jpeg",
		},
		{
			desc:                       "check file content industrial.jpeg",
			method:                     http.MethodGet,
			path:                       "/public/industrial.jpeg",
			statusCode:                 http.StatusOK,
			expectedBodyLength:         11228,
			expectedResponseHeaderType: "image/jpeg",
		},
		{
			desc:                       "check file content skross.png",
			method:                     http.MethodGet,
			path:                       "/public/skross.png",
			statusCode:                 http.StatusOK,
			expectedBodyLength:         76239,
			expectedResponseHeaderType: "image/png",
		},
		{
			desc:       "check public endpoint",
			method:     http.MethodGet,
			path:       "/public",
			statusCode: http.StatusNotFound,
		},
	}

	for it, tc := range tests {
		request, _ := http.NewRequest(tc.method, host+tc.path, bytes.NewBuffer(tc.body))
		request.Header.Set("Content-Type", "application/json")
		client := http.Client{}
		resp, err := client.Do(request)
		bodyBytes, _ := io.ReadAll(resp.Body)
		body := string(bodyBytes)
		assert.Nil(t, err, "TEST[%d], Failed.\n%s", it, tc.desc)
		assert.Equal(t, tc.statusCode, resp.StatusCode, "TEST[%d], Failed with Status Body.\n%s", it, tc.desc)
		if tc.expectedBody != "" {
			assert.Equal(t, tc.expectedBody, body, "TEST [%d], Failed with Expected Body. \n%s", it, tc.desc)
		}
		if tc.expectedBodyLength != 0 {
			contentLength, _ := strconv.Atoi(resp.Header.Get("Content-Length"))
			assert.Equal(t, tc.expectedBodyLength, contentLength, "TEST [%d], Failed at Content-Length.\n %s", it, tc.desc)
		}
		if tc.expectedResponseHeaderType != "" {
			assert.Equal(t, tc.expectedResponseHeaderType, resp.Header.Get("Content-Type"), "TEST [%d], Failed at Expected Content-Type.\n%s", it, tc.desc)
		}
	}
}
