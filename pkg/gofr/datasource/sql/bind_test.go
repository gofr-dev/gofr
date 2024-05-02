package sql

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func Test_Rebind(t *testing.T) {
	tests := []struct {
		desc     string
		dialect  string
		input    string
		expected string
	}{
		{
			desc:     "insert",
			dialect:  "mysql",
			input:    "INSERT INTO user (id, name) VALUES (?, ?)",
			expected: "INSERT INTO user (id, name) VALUES (?, ?)",
		},
		{
			desc:     "insert",
			dialect:  "postgres",
			input:    "INSERT INTO user (id, name) VALUES (?, ?)",
			expected: "INSERT INTO user (id, name) VALUES ($1, $2)",
		},
		{
			desc:     "insert",
			dialect:  "any-other-dialect",
			input:    "INSERT INTO user (id, name) VALUES (?, ?)",
			expected: "INSERT INTO user (id, name) VALUES (?, ?)",
		},
	}
	for i, tc := range tests {
		t.Run(tc.dialect+" "+tc.desc, func(t *testing.T) {
			actual := Rebind(tc.dialect, tc.input)
			assert.Equal(t, tc.expected, actual, "TEST[%d], Failed.\n%s", i, tc.desc)
		})
	}
}

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
