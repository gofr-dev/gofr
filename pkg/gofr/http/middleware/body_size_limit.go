package middleware

import (
	"encoding/json"
	"net/http"
)

const (
	// DefaultMaxBodySize is the default maximum request body size (10 MB)
	DefaultMaxBodySize = 10 << 20 // 10 MB
)

// BodySizeLimit is a middleware that limits the size of request bodies to prevent DoS attacks.
// It reads the maximum body size from the configuration or uses a default value.
func BodySizeLimit(maxSize int64) func(http.Handler) http.Handler {
	if maxSize <= 0 {
		maxSize = DefaultMaxBodySize
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {

			// Check Content-Length header if present
			if contentLength := r.ContentLength; contentLength > 0 {
				if contentLength > maxSize {
					respondWithError(w, NewRequestBodyTooLargeError(maxSize, contentLength))
					return
				}
			}

			// Wrap the request body with a LimitedReader to enforce the limit
			limitedBody := http.MaxBytesReader(w, r.Body, maxSize)
			r.Body = limitedBody

			// Call the next handler
			next.ServeHTTP(w, r)
		})
	}
}

// respondWithError writes an error response in JSON format.
func respondWithError(w http.ResponseWriter, err ErrorHTTP) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(err.StatusCode())

	response := map[string]any{
		"code":    err.StatusCode(),
		"status":  "ERROR",
		"message": err.Error(),
	}

	_ = json.NewEncoder(w).Encode(response)
}
