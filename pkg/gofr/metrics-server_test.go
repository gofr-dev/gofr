package gofr

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"

	"github.com/stretchr/testify/assert"

	"gofr.dev/pkg/log"
)

func TestMetricsServer(t *testing.T) {
	// Create the server
	srv := metricsServer(log.NewMockLogger(io.Discard), 2121, "/metrics")

	// Ensure that following routes are working on the server
	validURLs := []string{
		"/metrics",
		"/debug/pprof/",
		"/debug/pprof/profile",
		"/debug/pprof/cmdline",
		"/debug/pprof/symbol",
		"/debug/pprof/trace",
	}

	serverURL := "http://localhost:" + strconv.Itoa(2121)

	for _, u := range validURLs {
		r := httptest.NewRequest(http.MethodGet, serverURL+u, nil)
		rr := httptest.NewRecorder()
		srv.Handler.ServeHTTP(rr, r)

		assert.Equal(t, http.StatusOK, rr.Code)
	}
}
