package serrors

import (
	"errors"
	"strings"
	"testing"
)

func TestGetExternalError(t *testing.T) {
	tests := []struct {
		name     string
		input    *Error
		expected string
	}{
		{
			name:     "Nil Error Pointer",
			input:    nil,
			expected: "0 | NA",
		},
		{
			name: "Valid Error - OK",
			input: &Error{
				externalStatusCode: 200,
				externalMessage:    "OK",
			},
			expected: "200 | OK",
		},
		{
			name: "Valid Error - Not Found",
			input: &Error{
				externalStatusCode: 404,
				externalMessage:    "Not Found",
			},
			expected: "404 | Not Found",
		},
		{
			name: "Empty External Message",
			input: &Error{
				externalStatusCode: 500,
				externalMessage:    "",
			},
			expected: "500 | ",
		},
		{
			name: "Zero Status with Message",
			input: &Error{
				externalStatusCode: 0,
				externalMessage:    "Unknown Error",
			},
			expected: "0 | Unknown Error",
		},
		{
			name: "Negative Status Code",
			input: &Error{
				externalStatusCode: -1,
				externalMessage:    "Negative code",
			},
			expected: "-1 | Negative code",
		},
		{
			name: "Large Status Code",
			input: &Error{
				externalStatusCode: 9999000000099,
				externalMessage:    "Something big",
			},
			expected: "9999000000099 | Something big",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			actual := GetExternalError(tt.input)
			if actual != tt.expected {
				t.Errorf("GetExternalError() = %q, want %q", actual, tt.expected)
			}
		})
	}
}

func TestGetInternalError_AllCases(t *testing.T) {
	tests := []struct {
		name          string
		err           *Error
		addMeta       bool
		expectContain []string
		expectPanic   bool
	}{
		{
			name: "Full input with meta",
			err: &Error{
				cause:         errors.New("db timeout"), //nolint:err113 // appears due to the nature of table tests
				message:       "Database error",
				statusCode:    "E100",
				subStatusCode: "E101",
				level:         ERROR,
				meta:          map[string]any{"req": "abc"},
			},
			addMeta: true,
			expectContain: []string{
				"ERROR | E100 | E101 | Database error",
				`"req":"abc"`,
			},
		},
		{
			name: "Full input without meta",
			err: &Error{
				cause:         errors.New("error"), //nolint:err113 // appears due to the nature of table tests
				message:       "Error occurred",
				statusCode:    "X001",
				subStatusCode: "X002",
				level:         WARNING,
				meta:          map[string]any{"x": 1},
			},
			addMeta: false,
			expectContain: []string{
				"WARNING | X001 | X002 | Error occurred",
			},
		},
		{
			name: "Nil cause",
			err: &Error{
				cause:         nil,
				message:       "No cause",
				statusCode:    "Y001",
				subStatusCode: "Y002",
				level:         INFO,
			},
			addMeta: false,
			expectContain: []string{
				"INFO | Y001 | Y002 | No cause | Nil cause",
			},
		},
		{
			name: "Nil meta with addMeta=true",
			err: &Error{
				cause:         errors.New("oops"), //nolint:err113 // appears due to the nature of table tests
				message:       "Something",
				statusCode:    "Z001",
				subStatusCode: "Z002",
				level:         CRITICAL,
				meta:          nil,
			},
			addMeta: true,
			expectContain: []string{
				"CRITICAL | Z001 | Z002 | Something",
				"{", // Should include valid JSON, even if empty or null
			},
		},
		{
			name: "Unknown level value",
			err: &Error{
				cause:         errors.New("bad"), //nolint:err113 // appears due to the nature of table tests
				message:       "Corrupt",
				statusCode:    "U001",
				subStatusCode: "U002",
				level:         Level(99), // Unknown level
			},
			addMeta: false,
			expectContain: []string{
				"UNKNOWN | U001 | U002 | Corrupt",
			},
		},
		{
			name:        "Nil error struct (panic expected)",
			err:         nil,
			addMeta:     false,
			expectPanic: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if tc.expectPanic {
				defer func() {
					if r := recover(); r == nil {
						t.Errorf("Expected panic, got none")
					}
				}()
			}

			output := GetInternalError(tc.err, tc.addMeta)
			for _, fragment := range tc.expectContain {
				if !strings.Contains(output, fragment) {
					t.Errorf("Expected output to contain %q; got %s", fragment, output)
				}
			}
		})
	}
}
