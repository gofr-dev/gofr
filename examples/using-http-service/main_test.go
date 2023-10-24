package main

import (
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"gofr.dev/pkg/gofr/request"
	"gofr.dev/pkg/log"
)

func TestIntegration(t *testing.T) {
	ts := mockServer(t)
	defer ts.Close()

	t.Setenv("SAMPLE_API_URL", ts.URL)

	go main()
	time.Sleep(3 * time.Second)

	tests := []struct {
		desc       string
		endpoint   string
		statusCode int
	}{
		{"successful get request", "user/Vikash ", http.StatusOK},
		{"get with incomplete URL", "user/ ", http.StatusBadRequest},
		{"get with invalid URL", "dummyendpoint", http.StatusNotFound},
	}

	for i, tc := range tests {
		req, _ := request.NewMock(http.MethodGet, "http://localhost:9096/"+tc.endpoint, nil)
		c := http.Client{}

		resp, err := c.Do(req)
		if err != nil {
			t.Errorf("TEST[%v] Failed.\tHTTP request encountered Err: %v\n%s", i, err, tc.desc)
			continue
		}

		if resp.StatusCode != tc.statusCode {
			t.Errorf("TEST[%v] Failed.\tExpected %v\tGot %v\n%s", i, tc.statusCode, resp.StatusCode, tc.desc)
		}

		_ = resp.Body.Close()
	}
}

// mockServer mocks sample-api server
func mockServer(t *testing.T) *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Header().Set("Content-type", "application/json")

		_, err := w.Write([]byte(`{
				"data": {
        			"name": "Vikash",
        			"company": "ZopSmart"
    			}
			}`))

		if err != nil {
			t.Error("error in setting up mock server: failure in writing response")
		}
	}))
}

func Test_httpSvcRetry(t *testing.T) {
	logger := log.NewMockLogger(io.Discard)

	tests := []struct {
		desc       string
		err        error
		statusCode int
		count      int
		expOut     bool
	}{
		{"Making 1 retry attempt for http call", nil, http.StatusBadRequest, 1, true},
		{"Making 2 retry attempts for http call", nil, http.StatusBadRequest, 2, true},
		{"Making 3 retry attempts for http call", nil, http.StatusBadRequest, 3, true},
		{"success http call", nil, http.StatusOK, 0, false},
		{"retry failed error", errors.New("retry failed"), http.StatusBadRequest, 1, true},
	}

	for i, tc := range tests {
		output := httpSvcRetry(logger, tc.err, tc.statusCode, tc.count)

		assert.Equal(t, tc.expOut, output, "Test case [%d] failed.\n%s", i, tc.desc)
	}
}
