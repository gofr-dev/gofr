package response

import (
	"encoding/json"
	"net/http"

	"github.com/gofr-dev/gofr/pkg/errors"
)

// RespondWithProblem sends a RFC 7807 problem details response
func RespondWithProblem(w http.ResponseWriter, problem *errors.ProblemDetails) {
	w.Header().Set("Content-Type", "application/problem+json")
	w.WriteHeader(problem.Status)
	
	if err := json.NewEncoder(w).Encode(problem); err != nil {
		// Fallback to simple error if JSON encoding fails
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
	}
} 
