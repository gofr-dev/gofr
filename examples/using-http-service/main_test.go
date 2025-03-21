package main

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"gofr.dev/pkg/gofr"
	"gofr.dev/pkg/gofr/container"
	gofrHTTP "gofr.dev/pkg/gofr/http"
	"gofr.dev/pkg/gofr/logging"
	"gofr.dev/pkg/gofr/service"
	"gofr.dev/pkg/gofr/testutil"
)

var port int

func TestMain(m *testing.M) {
	os.Setenv("GOFR_TELEMETRY", "false")
	m.Run()
}

func Test_main(t *testing.T) {
	configs := testutil.NewServerConfigs(t)

	c := &http.Client{}

	go main()
	time.Sleep(100 * time.Millisecond)

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
				`"fact-checker":{"status":"DOWN","details":{"error":"service down","host":"catfact.ninja"}},` +
				`"name":"using-http-service","status":"DEGRADED","version":"dev"}}` + "\n",
			statusCode: 200,
		},
	}

	for i, tc := range testCases {
		req, _ := http.NewRequest(http.MethodGet, configs.HTTPHost+tc.path, nil)
		resp, err := c.Do(req)

		require.NoError(t, err, "TEST[%d], Failed.\n%s", i, tc.desc)

		bodyBytes, err := io.ReadAll(resp.Body)

		require.NoError(t, err, "TEST[%d], Failed.\n%s", i, tc.desc)

		assert.Equal(t, tc.expectedRes, string(bodyBytes), "TEST[%d], Failed.\n%s", i, tc.desc)

		assert.Equal(t, tc.statusCode, resp.StatusCode, "TEST[%d], Failed.\n%s", i, tc.desc)

		resp.Body.Close()
	}
}

func TestHTTPHandlerURLError(t *testing.T) {
	req, _ := http.NewRequestWithContext(context.Background(), http.MethodGet,
		fmt.Sprint("http://localhost:", port, "/handle"), bytes.NewBuffer([]byte(`{"key":"value"}`)))
	gofrReq := gofrHTTP.NewRequest(req)

	mockContainer, _ := container.NewMockContainer(t)

	ctx := &gofr.Context{Context: context.Background(), Request: gofrReq, Container: mockContainer}

	ctx.Container.Services = map[string]service.HTTP{"cat-facts": service.NewHTTPService("http://invalid", ctx.Logger, mockContainer.Metrics())}

	resp, err := Handler(ctx)

	assert.Nil(t, resp)
	require.Error(t, err)
}

func TestHTTPHandlerResponseUnmarshalError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// read request body
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{invalid body}`))
	}))
	defer server.Close()

	logger := logging.NewLogger(logging.DEBUG)

	req, _ := http.NewRequestWithContext(context.Background(), http.MethodGet, fmt.Sprint("http://localhost:", port, "/handle"), bytes.NewBuffer([]byte(`{"key":"value"}`)))

	gofrReq := gofrHTTP.NewRequest(req)

	ctx := &gofr.Context{Context: context.Background(),
		Request: gofrReq, Container: &container.Container{Logger: logger}}

	ctx.Container.Services = map[string]service.HTTP{"cat-facts": service.NewHTTPService(server.URL, ctx.Logger, nil)}

	resp, err := Handler(ctx)

	assert.Nil(t, resp)
	require.Error(t, err)
}
