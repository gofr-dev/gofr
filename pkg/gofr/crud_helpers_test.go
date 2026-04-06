package gofr

import (
	"reflect"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"gofr.dev/pkg/gofr/datasource/sql"
)

func Test_toSnakeCase(t *testing.T) {
	tests := []struct {
		desc     string
		input    string
		expected string
	}{
		{desc: "simple camelCase", input: "camelCase", expected: "camel_case"},
		{desc: "PascalCase", input: "PascalCase", expected: "pascal_case"},
		{desc: "already lowercase", input: "lowercase", expected: "lowercase"},
		{desc: "acronym at start", input: "HTTPServer", expected: "http_server"},
		{desc: "acronym in middle", input: "getHTTPResponse", expected: "get_http_response"},
		{desc: "single word uppercase", input: "ID", expected: "id"},
		{desc: "empty string", input: "", expected: ""},
		{desc: "single lowercase char", input: "a", expected: "a"},
		{desc: "single uppercase char", input: "A", expected: "a"},
		{desc: "multiple uppercase sequence", input: "UserID", expected: "user_id"},
	}

	for _, tc := range tests {
		t.Run(tc.desc, func(t *testing.T) {
			result := toSnakeCase(tc.input)
			assert.Equal(t, tc.expected, result)
		})
	}
}

func Test_parseSQLTag(t *testing.T) {
	tests := []struct {
		desc        string
		tag         reflect.StructTag
		expected    sql.FieldConstraints
		expectError bool
	}{
		{
			desc:     "empty tag",
			tag:      ``,
			expected: sql.FieldConstraints{},
		},
		{
			desc:     "auto_increment tag",
			tag:      `sql:"auto_increment"`,
			expected: sql.FieldConstraints{AutoIncrement: true},
		},
		{
			desc:     "not_null tag",
			tag:      `sql:"not_null"`,
			expected: sql.FieldConstraints{NotNull: true},
		},
		{
			desc:     "multiple tags",
			tag:      `sql:"auto_increment,not_null"`,
			expected: sql.FieldConstraints{AutoIncrement: true, NotNull: true},
		},
		{
			desc:     "case insensitive",
			tag:      `sql:"AUTO_INCREMENT"`,
			expected: sql.FieldConstraints{AutoIncrement: true},
		},
		{
			desc:        "invalid tag",
			tag:         `sql:"unknown_tag"`,
			expectError: true,
		},
		{
			desc:     "no sql tag present",
			tag:      `json:"name"`,
			expected: sql.FieldConstraints{},
		},
	}

	for _, tc := range tests {
		t.Run(tc.desc, func(t *testing.T) {
			result, err := parseSQLTag(tc.tag)

			if tc.expectError {
				require.Error(t, err)
				assert.ErrorIs(t, err, errInvalidSQLTag)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tc.expected, result)
			}
		})
	}
}

func Test_hasAutoIncrementID(t *testing.T) {
	tests := []struct {
		desc        string
		constraints map[string]sql.FieldConstraints
		expected    bool
	}{
		{
			desc:        "has auto increment",
			constraints: map[string]sql.FieldConstraints{"id": {AutoIncrement: true}},
			expected:    true,
		},
		{
			desc:        "no auto increment",
			constraints: map[string]sql.FieldConstraints{"id": {NotNull: true}},
			expected:    false,
		},
		{
			desc:        "empty constraints",
			constraints: map[string]sql.FieldConstraints{},
			expected:    false,
		},
		{
			desc:        "nil constraints",
			constraints: nil,
			expected:    false,
		},
		{
			desc: "auto increment in non-id field",
			constraints: map[string]sql.FieldConstraints{
				"id":      {NotNull: true},
				"counter": {AutoIncrement: true},
			},
			expected: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.desc, func(t *testing.T) {
			result := hasAutoIncrementID(tc.constraints)
			assert.Equal(t, tc.expected, result)
		})
	}
}
