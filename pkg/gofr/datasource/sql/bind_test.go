package sql

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func Test_BindType(t *testing.T) {
	tests := []struct {
		dialect  string
		expected BindVarType
	}{
		{
			dialect:  "mysql",
			expected: QUESTION,
		},
		{
			dialect:  "postgres",
			expected: DOLLAR,
		},
		{
			dialect:  "any-other-dialect",
			expected: UNKNOWN,
		},
	}
	for i, tc := range tests {
		t.Run(tc.dialect+" bind type", func(t *testing.T) {
			actual := bindType(tc.dialect)
			assert.Equal(t, tc.expected, actual, "TEST[%d], Failed.\n%s", i, tc.dialect+" bind type")
		})
	}
}

func Test_BindVar(t *testing.T) {
	tests := []struct {
		name     string
		dialect  string
		position int
		expected string
	}{
		{
			name:     "Postgres bind var",
			dialect:  dialectPostgres,
			position: 1,
			expected: "$1",
		},
		{
			name:     "MySQL bind var",
			dialect:  dialectMysql,
			position: 1,
			expected: "?",
		},
		{
			name:     "Unknown dialect bind var",
			dialect:  "unknown",
			position: 1,
			expected: "?",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			actual := bindVar(tc.dialect, tc.position)
			assert.Equal(t, tc.expected, actual)
		})
	}
}

func Test_Quote(t *testing.T) {
	tests := []struct {
		name     string
		dialect  string
		expected string
	}{
		{
			name:     "Postgres quote",
			dialect:  dialectPostgres,
			expected: quoteDouble,
		},
		{
			name:     "MySQL quote",
			dialect:  dialectMysql,
			expected: quoteBack,
		},
		{
			name:     "Unknown dialect quote",
			dialect:  "unknown",
			expected: quoteBack,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			actual := quote(tc.dialect)
			assert.Equal(t, tc.expected, actual)
		})
	}
}

func Test_QuotedString(t *testing.T) {
	tests := []struct {
		name     string
		q        string
		s        string
		expected string
	}{
		{
			name:     "Double quote",
			q:        quoteDouble,
			s:        "test",
			expected: `"test"`,
		},
		{
			name:     "Back quote",
			q:        quoteBack,
			s:        "test",
			expected: "`test`",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			actual := quotedString(tc.q, tc.s)
			assert.Equal(t, tc.expected, actual)
		})
	}
}
