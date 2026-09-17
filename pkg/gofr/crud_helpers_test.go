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
		// A digit ends a word, so an uppercase letter following one starts a new one. All five
		// disagreed with the regex-based converters on development. Only the first two still did
		// after the digit passthrough alone -- the other three were already right by then, and are
		// here to hold the rule rather than to record a past difference.
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
		// The bounds the two helpers turn on. Each is a single comparison, and a mutation that
		// moves one is invisible unless a case sits exactly on it -- verified by moving all five
		// and checking which cases fail.
		//
		// "ABé" pins the `r <= 'z'` half of isLower against being dropped, which is the whole
		// reason isLower is written out rather than left as the old `r >= 'a'`. The byte read at
		// str[i+1] there is 0xC3, the UTF-8 LEAD byte of 'é' -- numerically above 'a', so the old
		// guard called it lowercase and inserted a word break that is not there. (A continuation
		// byte, 0x80-0xBF, reaches the same wrong answer through str[i-1]; "CaféB" is that case.)
		{desc: "capital before non-ASCII, isLower upper bound not dropped", input: "ABé", expected: "abé"},
		{desc: "capital after 'z', isLower upper bound", input: "AzB", expected: "az_b"},
		{desc: "capital after 'a', isLower lower bound", input: "AaB", expected: "aa_b"},
		{desc: "capital after '0', digit range lower bound", input: "A0B", expected: "a0_b"},
		{desc: "capital after '9', digit range upper bound", input: "X509CERT", expected: "x509_cert"},
		// The loop ranges over runes but indexes bytes, so `length` must stay len(str). With a
		// multi-byte rune before the capital the two disagree: here the 'A' sits at byte index 2 of
		// four, and a rune length of three would put it at the end and suppress the underscore.
		{desc: "multi-byte rune before a capital, byte length", input: "éAb", expected: "é_ab"},
	}

	for _, tc := range tests {
		t.Run(tc.desc, func(t *testing.T) {
			result := toSnakeCase(tc.input)
			assert.Equal(t, tc.expected, result)
		})
	}
}

// Test_toSnakeCase_MatchesSQLConverter pins the agreement between this loop and the exported
// sql.ToSnakeCase, which DB.Select uses to map result columns back onto struct fields. The two
// are separate implementations of the same rule, and nothing but this test stops them drifting:
// before the digit fix they disagreed on 26,260 of the identifiers below, which meant an INSERT
// wrote one column name and a Select looked for another.
//
// ASCII only. The regex converter finishes with a Unicode-aware strings.ToLower and this loop
// only touches A-Z, so a non-ASCII capital still diverges -- a known gap, not a drift.
func Test_toSnakeCase_MatchesSQLConverter(t *testing.T) {
	const alphabet = "abzABZ019_"

	n := len(alphabet)
	identifiers := make([]string, 0, n+n*n+n*n*n)

	for _, a := range alphabet {
		identifiers = append(identifiers, string(a))

		for _, b := range alphabet {
			identifiers = append(identifiers, string(a)+string(b))

			for _, c := range alphabet {
				identifiers = append(identifiers, string(a)+string(b)+string(c))
			}
		}
	}

	identifiers = append(identifiers,
		"UserID", "HTTPServer", "OAuth2Client", "X509Cert", "Order2FA", "Base64Encoder",
		"BillingAddressLine1", "SubscriptionExpiryTimestamp", "IsActive", "CreatedAt")

	for _, id := range identifiers {
		if got, want := toSnakeCase(id), sql.ToSnakeCase(id); got != want {
			t.Errorf("toSnakeCase(%q) = %q, sql.ToSnakeCase = %q -- the two converters have drifted",
				id, got, want)
		}
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
