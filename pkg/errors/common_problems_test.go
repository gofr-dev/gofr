package errors

import (
	"net/http"
	"testing"
)

func TestNewValidationProblem(t *testing.T) {
	detail := "Invalid input"
	problem := NewValidationProblem(detail)

	tests := []struct {
		name     string
		got      interface{}
		expected interface{}
	}{
		{"type", problem.Type, ValidationProblem},
		{"status", problem.Status, http.StatusBadRequest},
		{"title", problem.Title, "Validation Error"},
		{"detail", problem.Detail, detail},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.got != tt.expected {
				t.Errorf("NewValidationProblem() %s = %v, want %v", tt.name, tt.got, tt.expected)
			}
		})
	}
}

func TestNewAuthenticationProblem(t *testing.T) {
	detail := "Invalid credentials"
	problem := NewAuthenticationProblem(detail)

	tests := []struct {
		name     string
		got      interface{}
		expected interface{}
	}{
		{"type", problem.Type, AuthenticationProblem},
		{"status", problem.Status, http.StatusUnauthorized},
		{"title", problem.Title, "Authentication Required"},
		{"detail", problem.Detail, detail},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.got != tt.expected {
				t.Errorf("NewAuthenticationProblem() %s = %v, want %v", tt.name, tt.got, tt.expected)
			}
		})
	}
}

func TestNewAuthorizationProblem(t *testing.T) {
	detail := "Insufficient permissions"
	problem := NewAuthorizationProblem(detail)

	tests := []struct {
		name     string
		got      interface{}
		expected interface{}
	}{
		{"type", problem.Type, AuthorizationProblem},
		{"status", problem.Status, http.StatusForbidden},
		{"title", problem.Title, "Authorization Failed"},
		{"detail", problem.Detail, detail},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.got != tt.expected {
				t.Errorf("NewAuthorizationProblem() %s = %v, want %v", tt.name, tt.got, tt.expected)
			}
		})
	}
}

func TestNewNotFoundProblem(t *testing.T) {
	detail := "Resource not found"
	problem := NewNotFoundProblem(detail)

	tests := []struct {
		name     string
		got      interface{}
		expected interface{}
	}{
		{"type", problem.Type, NotFoundProblem},
		{"status", problem.Status, http.StatusNotFound},
		{"title", problem.Title, "Resource Not Found"},
		{"detail", problem.Detail, detail},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.got != tt.expected {
				t.Errorf("NewNotFoundProblem() %s = %v, want %v", tt.name, tt.got, tt.expected)
			}
		})
	}
} 
