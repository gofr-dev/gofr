package mongo

import (
	"bytes"
	"fmt"
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestPrettyPrint(t *testing.T) {
	tests := []struct {
		name     string
		queryLog QueryLog
		expected string
	}{
		{
			name: "All fields present",
			queryLog: QueryLog{
				Query:      "find",
				Duration:   12345,
				Collection: "users",
				Filter:     map[string]string{"name": "John"},
				ID:         "123",
				Update:     map[string]string{"$set": "Doe"},
			},
			expected: "name:John",
		},
		{
			name: "Missing optional fields",
			queryLog: QueryLog{
				Query:    "insert",
				Duration: 6789,
			},
			expected: "MONGO",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var buf bytes.Buffer
			tc.queryLog.PrettyPrint(&buf)
			fmt.Println(buf.String())
			fmt.Println(tc.expected)

			assert.Contains(t, buf.String(), tc.expected)
		})
	}
}
