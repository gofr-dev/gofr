package response

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gofr-dev/gofr/pkg/errors"
)

func TestRespondWithProblem(t *testing.T) {
	tests := []struct {
		name           string
		problem        *errors.ProblemDetails
		expectedStatus int
		expectedBody   map[string]interface{}
	}{
		{
			name: "basic problem",
			problem: errors.NewProblemDetails(
				http.StatusBadRequest,
				"Test Error",
				"Something went wrong",
			),
			expectedStatus: http.StatusBadRequest,
			expectedBody: map[string]interface{}{
				"type":   "about:blank",
				"title":  "Test Error",
				"status": float64(http.StatusBadRequest),
				"detail": "Something went wrong",
			},
		},
		{
			name: "problem with extensions",
			problem: errors.NewProblemDetails(
				http.StatusBadRequest,
				"Test Error",
				"Something went wrong",
			).WithExtension("code", 123),
			expectedStatus: http.StatusBadRequest,
			expectedBody: map[string]interface{}{
				"type":   "about:blank",
				"title":  "Test Error",
				"status": float64(http.StatusBadRequest),
				"detail": "Something went wrong",
				"code":   float64(123),
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			RespondWithProblem(w, tt.problem)

			// Check status code
			if w.Code != tt.expectedStatus {
				t.Errorf("RespondWithProblem() status = %v, want %v", w.Code, tt.expectedStatus)
			}

			// Check Content-Type
			contentType := w.Header().Get("Content-Type")
			if contentType != "application/problem+json" {
				t.Errorf("RespondWithProblem() Content-Type = %v, want application/problem+json", contentType)
			}

			// Check body
			var got map[string]interface{}
			if err := json.NewDecoder(w.Body).Decode(&got); err != nil {
				t.Fatalf("Failed to decode response body: %v", err)
			}

			if !mapsEqual(got, tt.expectedBody) {
				t.Errorf("RespondWithProblem() body = %v, want %v", got, tt.expectedBody)
			}
		})
	}
}

// Helper function to compare maps
func mapsEqual(m1, m2 map[string]interface{}) bool {
	if len(m1) != len(m2) {
		return false
	}
	for k, v1 := range m1 {
		v2, ok := m2[k]
		if !ok {
			return false
		}
		if v1 != v2 {
			return false
		}
	}
	return true
} 
