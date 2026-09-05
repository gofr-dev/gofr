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
		{desc: "trailing digit", input: "User1", expected: "user1"},
		{desc: "digit after leading acronym", input: "S3Bucket", expected: "s3_bucket"},
		{desc: "digits within a word", input: "Base64Encoder", expected: "base64_encoder"},
		{desc: "digit between words", input: "User2Name", expected: "user2_name"},
		// The `char >= 'a'` guard corrupted every rune below 'a', not only digits: '_' (95)
		// became DEL (127), putting a control character in a SQL identifier.
		{desc: "underscore already present", input: "Field_1", expected: "field_1"},
		{desc: "leading underscore", input: "_User", expected: "__user"},
		// A digit ends a word, so an uppercase letter following one starts a new one. These are
		// the cases where this implementation used to disagree with the Cassandra datasource's
		// regex-based toSnakeCase; it now matches it.
		{desc: "digit then acronym", input: "Order2FA", expected: "order2_fa"},
		{desc: "digit between single capitals", input: "A2B", expected: "a2_b"},
		{desc: "digit then lowercase word", input: "X509Cert", expected: "x509_cert"},
		{desc: "acronym then digit", input: "HTTP2Server", expected: "http2_server"},
		{desc: "mixed caps, digit, word", input: "OAuth2Client", expected: "o_auth2_client"},
		// ...but only on the left. A trailing digit does not split a leading acronym.
		{desc: "acronym with trailing digit", input: "AB2", expected: "ab2"},
		{desc: "single capital then digit", input: "A1", expected: "a1"},
		{desc: "acronym then digit, no word", input: "ID2", expected: "id2"},
		// Non-ASCII passes through untouched; it is neither uppercase ASCII nor a word boundary.
		{desc: "non-ASCII leading rune", input: "Ünicode", expected: "Ünicode"},
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
