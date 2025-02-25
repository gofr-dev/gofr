package errors

import "net/http"

// Common problem type URIs
const (
	ValidationProblem    = "https://api.example.com/problems/validation"
	AuthenticationProblem = "https://api.example.com/problems/authentication"
	AuthorizationProblem = "https://api.example.com/problems/authorization"
	NotFoundProblem      = "https://api.example.com/problems/not-found"
)

// NewValidationProblem creates a new validation problem details
func NewValidationProblem(detail string) *ProblemDetails {
	return NewProblemDetails(
		http.StatusBadRequest,
		"Validation Error",
		detail,
	).WithType(ValidationProblem)
}

// NewAuthenticationProblem creates a new authentication problem details
func NewAuthenticationProblem(detail string) *ProblemDetails {
	return NewProblemDetails(
		http.StatusUnauthorized,
		"Authentication Required",
		detail,
	).WithType(AuthenticationProblem)
}

// NewAuthorizationProblem creates a new authorization problem details
func NewAuthorizationProblem(detail string) *ProblemDetails {
	return NewProblemDetails(
		http.StatusForbidden,
		"Authorization Failed",
		detail,
	).WithType(AuthorizationProblem)
}

// NewNotFoundProblem creates a new not found problem details
func NewNotFoundProblem(detail string) *ProblemDetails {
	return NewProblemDetails(
		http.StatusNotFound,
		"Resource Not Found",
		detail,
	).WithType(NotFoundProblem)
} 
