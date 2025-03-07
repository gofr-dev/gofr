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
func NewValidationProblem(detail string, options ...ProblemOption) *ProblemDetails {
	// Start with the base validation problem options
	baseOptions := []ProblemOption{
		WithType(ValidationProblem),
		WithStatus(http.StatusBadRequest),
		WithTitle("Validation Error"),
		WithDetail(detail),
	}
	
	// Combine with any additional options
	return NewProblemDetails(append(baseOptions, options...)...)
}

// NewAuthenticationProblem creates a new authentication problem details
func NewAuthenticationProblem(detail string, options ...ProblemOption) *ProblemDetails {
	baseOptions := []ProblemOption{
		WithType(AuthenticationProblem),
		WithStatus(http.StatusUnauthorized),
		WithTitle("Authentication Required"),
		WithDetail(detail),
	}
	
	return NewProblemDetails(append(baseOptions, options...)...)
}

// NewAuthorizationProblem creates a new authorization problem details
func NewAuthorizationProblem(detail string, options ...ProblemOption) *ProblemDetails {
	baseOptions := []ProblemOption{
		WithType(AuthorizationProblem),
		WithStatus(http.StatusForbidden),
		WithTitle("Authorization Failed"),
		WithDetail(detail),
	}
	
	return NewProblemDetails(append(baseOptions, options...)...)
}

// NewNotFoundProblem creates a new not found problem details
func NewNotFoundProblem(detail string, options ...ProblemOption) *ProblemDetails {
	baseOptions := []ProblemOption{
		WithType(NotFoundProblem),
		WithStatus(http.StatusNotFound),
		WithTitle("Resource Not Found"),
		WithDetail(detail),
	}
	
	return NewProblemDetails(append(baseOptions, options...)...)
} 
