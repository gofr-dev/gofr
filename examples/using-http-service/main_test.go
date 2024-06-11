package main

import (
	"bytes"
	"context"
	"fmt"
	"go.uber.org/mock/gomock"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"gofr.dev/pkg/gofr"
	"gofr.dev/pkg/gofr/container"
	gofrHTTP "gofr.dev/pkg/gofr/http"
	"gofr.dev/pkg/gofr/logging"
	"gofr.dev/pkg/gofr/service"
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

func TestHTTPHandlerURLError(t *testing.T) {
	logger := logging.NewLogger(logging.DEBUG)

	req, _ := http.NewRequestWithContext(context.Background(), http.MethodGet, "http://localhost:5000/handle", bytes.NewBuffer([]byte(`{"key":"value"}`)))

	gofrReq := gofrHTTP.NewRequest(req)

	ctx := &gofr.Context{Context: context.Background(),
		Request: gofrReq, Container: &container.Container{Logger: logger}}

	ctrl := gomock.NewController(t)
	mockMetrics := service.NewMockMetrics(ctrl)

	mockMetrics.EXPECT().RecordHistogram(ctx, "app_http_service_response", gomock.Any(), "path", gomock.Any(),
		"method", http.MethodGet, "status", fmt.Sprintf("%v", http.StatusInternalServerError))

	ctx.Container.Services = map[string]service.HTTP{"cat-facts": service.NewHTTPService("http://invalid", ctx.Logger, mockMetrics)}

	resp, err := Handler(ctx)

	assert.Nil(t, resp)
	assert.NotNil(t, err)
}

func TestHTTPHandlerResponseUnmarshalError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// read request body
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{invalid body}`))
	}))
	defer server.Close()

	logger := logging.NewLogger(logging.DEBUG)

	req, _ := http.NewRequestWithContext(context.Background(), http.MethodGet, "http://localhost:5000/handle", bytes.NewBuffer([]byte(`{"key":"value"}`)))

	gofrReq := gofrHTTP.NewRequest(req)

	ctx := &gofr.Context{Context: context.Background(),
		Request: gofrReq, Container: &container.Container{Logger: logger}}

	ctx.Container.Services = map[string]service.HTTP{"cat-facts": service.NewHTTPService(server.URL, ctx.Logger, nil)}

	resp, err := Handler(ctx)

	assert.Nil(t, resp)
	assert.NotNil(t, err)
}
