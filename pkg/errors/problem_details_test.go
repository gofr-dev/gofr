package errors

import (
	"encoding/json"
	"testing"
)

func TestProblemDetails_Error(t *testing.T) {
	tests := []struct {
		name     string
		problem  *ProblemDetails
		expected string
	}{
		{
			name: "basic error string",
			problem: NewProblemDetails(
				WithTitle("Test Error"),
				WithDetail("Something went wrong"),
			),
			expected: "Test Error: Something went wrong",
		},
		{
			name: "empty title",
			problem: NewProblemDetails(
				WithDetail("Just details"),
			),
			expected: ": Just details",
		},
		{
			name: "empty detail",
			problem: NewProblemDetails(
				WithTitle("Just title"),
			),
			expected: "Just title: ",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.problem.Error(); got != tt.expected {
				t.Errorf("ProblemDetails.Error() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestProblemDetails_MarshalJSON(t *testing.T) {
	tests := []struct {
		name     string
		problem  *ProblemDetails
		expected string
		wantErr  bool
	}{
		{
			name: "basic fields",
			problem: NewProblemDetails(
				WithType("https://example.com/problems/test"),
				WithTitle("Test Error"),
				WithStatus(400),
				WithDetail("Something went wrong"),
			),
			expected: `{"type":"https://example.com/problems/test","title":"Test Error","status":400,"detail":"Something went wrong"}`,
		},
		{
			name: "with extensions",
			problem: NewProblemDetails(
				WithType("https://example.com/problems/test"),
				WithTitle("Test Error"),
				WithStatus(400),
				WithDetail("Something went wrong"),
				WithExtension("extra", "value"),
				WithExtension("code", 123),
			),
			expected: `{"type":"https://example.com/problems/test","title":"Test Error","status":400,"detail":"Something went wrong","extra":"value","code":123}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := json.Marshal(tt.problem)
			if (err != nil) != tt.wantErr {
				t.Errorf("ProblemDetails.MarshalJSON() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			// Compare JSON objects instead of raw strings to handle field ordering
			var gotMap, expectedMap map[string]interface{}
			if err := json.Unmarshal(got, &gotMap); err != nil {
				t.Fatalf("Failed to unmarshal result: %v", err)
			}
			if err := json.Unmarshal([]byte(tt.expected), &expectedMap); err != nil {
				t.Fatalf("Failed to unmarshal expected: %v", err)
			}

			if !mapsEqual(gotMap, expectedMap) {
				t.Errorf("ProblemDetails.MarshalJSON() = %v, want %v", string(got), tt.expected)
			}
		})
	}
}

func TestNewProblemDetails(t *testing.T) {
	problem := NewProblemDetails(
		WithStatus(400),
		WithTitle("Test Error"),
		WithDetail("Test Detail"),
	)

	if problem.Type != "about:blank" {
		t.Errorf("Expected Type to be 'about:blank', got %v", problem.Type)
	}
	if problem.Title != "Test Error" {
		t.Errorf("Expected Title to be 'Test Error', got %v", problem.Title)
	}
	if problem.Status != 400 {
		t.Errorf("Expected Status to be 400, got %v", problem.Status)
	}
	if problem.Detail != "Test Detail" {
		t.Errorf("Expected Detail to be 'Test Detail', got %v", problem.Detail)
	}
	if problem.Extensions == nil {
		t.Error("Expected Extensions to be initialized")
	}
}

func TestProblemOptions(t *testing.T) {
	problem := NewProblemDetails()

	// Test WithType
	WithType("https://example.com/problems/test")(problem)
	if problem.Type != "https://example.com/problems/test" {
		t.Errorf("WithType() failed, got %v", problem.Type)
	}

	// Test WithInstance
	WithInstance("/resources/123")(problem)
	if problem.Instance != "/resources/123" {
		t.Errorf("WithInstance() failed, got %v", problem.Instance)
	}

	// Test WithExtension
	WithExtension("code", 123)(problem)
	if val, ok := problem.Extensions["code"]; !ok || val != 123 {
		t.Errorf("WithExtension() failed, got %v", problem.Extensions["code"])
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
