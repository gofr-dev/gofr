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
