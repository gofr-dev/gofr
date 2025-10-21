// file: middleware/request_logger.go
package middleware

import (
    "log"
    "net/http"
    "time"
)

// RequestLogger logs the HTTP method, path, and execution time for each request
func RequestLogger(next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        start := time.Now()
        next.ServeHTTP(w, r)
        log.Printf("Method: %s, Path: %s, Duration: %s", r.Method, r.URL.Path, time.Since(start))
    })
}
// file: middleware/request_logger_test.go
package middleware

import (
    "net/http"
    "net/http/httptest"
    "testing"
)

func TestRequestLogger(t *testing.T) {
    handler := RequestLogger(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        w.WriteHeader(http.StatusOK)
    }))

    req := httptest.NewRequest("GET", "/test", nil)
    w := httptest.NewRecorder()
    handler.ServeHTTP(w, req)

    if w.Result().StatusCode != http.StatusOK {
        t.Errorf("expected status 200, got %d", w.Result().StatusCode)
    }
}
// Example usage in your server setup (e.g., main.go or router.go)
package main

import (
    "net/http"
    "gofr-dev/gofr/middleware"
)

func main() {
    mux := http.NewServeMux()
    mux.Handle("/api/", middleware.RequestLogger(http.HandlerFunc(apiHandler)))
    http.ListenAndServe(":8080", mux)
}

func apiHandler(w http.ResponseWriter, r *http.Request) {
    w.Write([]byte("Hello GoFr!"))
}
