package errors

import (
	"encoding/json"
	"fmt"
	"net/http"
)

// ProblemDetails implements RFC 7807 for HTTP API Problem Details
type ProblemDetails struct {
	// Type is a URI reference that identifies the problem type
	Type string `json:"type,omitempty"`
	
	// Title is a short, human-readable summary of the problem type
	Title string `json:"title,omitempty"`
	
	// Status is the HTTP status code
	Status int `json:"status,omitempty"`
	
	// Detail is a human-readable explanation specific to this occurrence of the problem
	Detail string `json:"detail,omitempty"`
	
	// Instance is a URI reference that identifies the specific occurrence of the problem
	Instance string `json:"instance,omitempty"`
	
	// Extensions holds additional properties of the problem detail
	Extensions map[string]interface{} `json:"-"`
}

// Error implements the error interface
func (p *ProblemDetails) Error() string {
	return fmt.Sprintf("%s: %s", p.Title, p.Detail)
}

// MarshalJSON implements custom JSON marshaling to include extensions
func (p *ProblemDetails) MarshalJSON() ([]byte, error) {
	type Alias ProblemDetails
	
	// Create a map for the base fields
	m := make(map[string]interface{})
	
	// Marshal the base structure
	base, err := json.Marshal((*Alias)(p))
	if err != nil {
		return nil, err
	}
	
	// Unmarshal into the map
	if err := json.Unmarshal(base, &m); err != nil {
		return nil, err
	}
	
	// Add extension fields
	for k, v := range p.Extensions {
		m[k] = v
	}
	
	return json.Marshal(m)
}

// NewProblemDetails creates a new ProblemDetails with all fields set directly
// This simplifies error creation by allowing all fields to be set at once
func NewProblemDetails(options ...ProblemOption) *ProblemDetails {
	// Create problem with default values
	p := &ProblemDetails{
		Type:       "about:blank", // default as per RFC 7807
		Extensions: make(map[string]interface{}),
	}
	
	// Apply all options
	for _, option := range options {
		option(p)
	}
	
	return p
}

// ProblemOption defines a function that configures a ProblemDetails
type ProblemOption func(*ProblemDetails)

// WithType sets the type URI
func WithType(typeURI string) ProblemOption {
	return func(p *ProblemDetails) {
		p.Type = typeURI
	}
}

// WithTitle sets the title
func WithTitle(title string) ProblemOption {
	return func(p *ProblemDetails) {
		p.Title = title
	}
}

// WithStatus sets the HTTP status code
func WithStatus(status int) ProblemOption {
	return func(p *ProblemDetails) {
		p.Status = status
	}
}

// WithDetail sets the detail message
func WithDetail(detail string) ProblemOption {
	return func(p *ProblemDetails) {
		p.Detail = detail
	}
}

// WithInstance sets the instance URI
func WithInstance(instance string) ProblemOption {
	return func(p *ProblemDetails) {
		p.Instance = instance
	}
}

// WithExtension adds an extension field
func WithExtension(key string, value interface{}) ProblemOption {
	return func(p *ProblemDetails) {
		if p.Extensions == nil {
			p.Extensions = make(map[string]interface{})
		}
		p.Extensions[key] = value
	}
} 
