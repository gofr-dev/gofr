package arangodb

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/assert"
)

func Test_PrettyPrint(t *testing.T) {
	queryLog := QueryLog{
		Query:      "",
		Duration:   12345,
		Database:   "test",
		Collection: "test",
		Filter:     true,
		ID:         "12345",
		Operation:  "getDocument",
	}
	expected := "getDocument"

	var buf bytes.Buffer

	queryLog.PrettyPrint(&buf)

	assert.Contains(t, buf.String(), expected)
}

func Test_PrettyPrint_DefaultsNilFilterAndID(t *testing.T) {
	tests := []struct {
		desc     string
		queryLog QueryLog
		expected string
	}{
		{
			desc:     "nil filter and id print as empty",
			queryLog: QueryLog{Operation: "dropDB", Database: "testDB"},
			expected: "dropDB",
		},
		{
			desc:     "set filter and id are kept",
			queryLog: QueryLog{Operation: "getDocument", Database: "testDB", Filter: true, ID: "42"},
			expected: "testDB true 42",
		},
	}

	for _, tc := range tests {
		t.Run(tc.desc, func(t *testing.T) {
			var buf bytes.Buffer

			tc.queryLog.PrettyPrint(&buf)

			assert.Contains(t, buf.String(), tc.expected)
			assert.NotNil(t, tc.queryLog.Filter)
			assert.NotNil(t, tc.queryLog.ID)
		})
	}
}
