package cassandra

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/assert"
)

func Test_PrettyPrint(t *testing.T) {
	queryLog := QueryLog{
		Query:    "sample query",
		Duration: 12345,
	}
	expected := "sample query"

	var buf bytes.Buffer

	queryLog.PrettyPrint(&buf)

	assert.Contains(t, buf.String(), expected)
}

func Test_Clean(t *testing.T) {
	testCases := []struct {
		desc     string
		input    string
		expected string
	}{
		{"multiple spaces", "   multiple   spaces   ", "multiple spaces"},
		{"leading and trailing", "leading and trailing   ", "leading and trailing"},
		{"mixed white spaces", "   mixed\twhite\nspaces", "mixed white spaces"},
		{"single word", "singleword", "singleword"},
		{"empty string", "", ""},
		{"empty string with spaces", "   ", ""},
	}

	for i, tc := range testCases {
		t.Run(tc.input, func(t *testing.T) {
			result := clean(tc.input)
			assert.Equal(t, tc.expected, result, "TEST[%d], Failed.\n%s", i, tc.desc)
		})
	}
}
