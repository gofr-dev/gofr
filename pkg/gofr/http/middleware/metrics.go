package middleware

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"gofr.dev/pkg/gofr/metrics"
)

func Metrics(m metrics.Manager) func(inner http.Handler) http.Handler {
	return func(inner http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()

			srw := &StatusResponseWriter{ResponseWriter: w}

			// this has to be called in the end so that status code is populated
			defer func(res *StatusResponseWriter, req *http.Request) {
				duration := time.Since(start)

				m.RecordHistogram(context.Background(), "app_http_response", duration.Seconds(),
					"path", r.URL.Path, "method", req.Method, "status", fmt.Sprintf("%d", res.status))
			}(srw, r)

			inner.ServeHTTP(srw, r)
		})
	}
}
