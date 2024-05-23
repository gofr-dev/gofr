package middleware

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/gorilla/mux"
)

type metrics interface {
	IncrementCounter(ctx context.Context, name string, labels ...string)
	DeltaUpDownCounter(ctx context.Context, name string, value float64, labels ...string)
	RecordHistogram(ctx context.Context, name string, value float64, labels ...string)
	SetGauge(name string, value float64, labels ...string)
}

// Metrics is a middleware that records request response time metrics using the provided metrics interface.
func Metrics(metrics metrics) func(inner http.Handler) http.Handler {
	return func(inner http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()

			srw := &StatusResponseWriter{ResponseWriter: w}

			path, _ := mux.CurrentRoute(r).GetPathTemplate()
			path = strings.TrimSuffix(path, "/")

			// this has to be called in the end so that status code is populated
			defer func(res *StatusResponseWriter, req *http.Request) {
				duration := time.Since(start)

				metrics.RecordHistogram(context.Background(), "app_http_response", duration.Seconds(),
					"path", path, "method", req.Method, "status", fmt.Sprintf("%d", res.status))
			}(srw, r)

			inner.ServeHTTP(srw, r)
		})
	}
}
