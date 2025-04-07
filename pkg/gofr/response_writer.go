package gofr

import "net/http"

// responseWriter wraps http.ResponseWriter to capture errors
type responseWriter struct {
	http.ResponseWriter
	err error
}

// Error captures the error for later handling
func (w *responseWriter) Error(err error) {
	w.err = err
}

// GetError returns the captured error
func (w *responseWriter) GetError() error {
	return w.err
} 
