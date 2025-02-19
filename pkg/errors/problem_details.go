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

// NewProblemDetails creates a new ProblemDetails with common defaults
func NewProblemDetails(status int, title, detail string) *ProblemDetails {
	return &ProblemDetails{
		Type:       "about:blank", // default as per RFC 7807
		Title:      title,
		Status:     status,
		Detail:     detail,
		Extensions: make(map[string]interface{}),
	}
}

// WithType sets the type URI and returns the ProblemDetails
func (p *ProblemDetails) WithType(typeURI string) *ProblemDetails {
	p.Type = typeURI
	return p
}

// WithInstance sets the instance URI and returns the ProblemDetails
func (p *ProblemDetails) WithInstance(instance string) *ProblemDetails {
	p.Instance = instance
	return p
}

// WithExtension adds an extension field and returns the ProblemDetails
func (p *ProblemDetails) WithExtension(key string, value interface{}) *ProblemDetails {
	if p.Extensions == nil {
		p.Extensions = make(map[string]interface{})
	}
	p.Extensions[key] = value
	return p
} 
