package gofr

import (
	"net/http"
	
	"github.com/gofr-dev/gofr/pkg/errors"
	"github.com/gofr-dev/gofr/pkg/http/response"
)

// ErrorHandler converts errors to appropriate HTTP responses
func ErrorHandler(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Create a response wrapper to capture errors
		rw := &responseWriter{ResponseWriter: w}
		
		// Call the next handler
		next.ServeHTTP(rw, r)
		
		// If there's an error, convert it to problem details
		if rw.err != nil {
			var problem *errors.ProblemDetails
			
			// Check if error is already a ProblemDetails
			if pd, ok := rw.err.(*errors.ProblemDetails); ok {
				problem = pd
			} else {
				// Convert standard error to ProblemDetails
				problem = errors.NewProblemDetails(
					http.StatusInternalServerError,
					"Internal Server Error",
					rw.err.Error(),
				)
			}
			
			response.RespondWithProblem(w, problem)
		}
	})
} 
