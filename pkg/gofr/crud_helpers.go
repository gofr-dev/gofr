package gofr

import (
	"fmt"
	"reflect"
	"strings"

	"gofr.dev/pkg/gofr/datasource/sql"
)

func getTableName(object any, structName string) string {
	if v, ok := object.(TableNameOverrider); ok {
		return v.TableName()
	}

	return toSnakeCase(structName)
}

func getRestPath(object any, structName string) string {
	if v, ok := object.(RestPathOverrider); ok {
		return v.RestPath()
	}

	return strings.ToLower(structName)
}

func hasAutoIncrementID(constraints map[string]sql.FieldConstraints) bool {
	for _, constraint := range constraints {
		if constraint.AutoIncrement {
			return true
		}
	}

	return false
}

func parseSQLTag(inputTags reflect.StructTag) (sql.FieldConstraints, error) {
	var constraints sql.FieldConstraints

	sqlTag := inputTags.Get("sql")
	if sqlTag == "" {
		return constraints, nil
	}

	tags := strings.Split(sqlTag, ",")

	for _, tag := range tags {
		tag = strings.ToLower(tag) // Convert to lowercase for case-insensitivity

		switch tag {
		case "auto_increment":
			constraints.AutoIncrement = true
		case "not_null":
			constraints.NotNull = true
		default:
			return constraints, fmt.Errorf("%w: %s", errInvalidSQLTag, tag)
		}
	}

	return constraints, nil
}

// snakeCaseHeadroom is how much room over the input length toSnakeCase reserves for the
// underscores it inserts.
//
// Grow rounds the capacity it takes up to a size class (8, 16, 24, 32, ...), so Grow(len(str))
// already carries up to seven spare bytes and most names never regrow without this. What the
// headroom rescues is a name whose LENGTH lands exactly on a size class and whose output does
// not fit: "IsActive" is 8 bytes in and 9 out, so Grow(8) takes an 8-byte class and reallocates,
// while Grow(8+4) takes 16 and does not. Measured with testing.AllocsPerRun over a corpus of
// realistic field names, Grow(len) needs a second allocation for three of them and Grow(len+4)
// for none.
const snakeCaseHeadroom = 4

// toSnakeCase converts a Go identifier to the snake_case name used for a table or column.
//
// Only uppercase ASCII letters are special: they are lowercased, and -- when they begin a new
// word -- separated from what comes before with an underscore. Not every capital does: "IDCard"
// is "id_card", where the D begins no word and takes no underscore.
//
// Every other rune -- digits, underscores, punctuation, non-ASCII -- passes through untouched.
// The rule is stated positively on purpose. The earlier guard was
// `char >= 'a'`, which relies on an ordering accident, and everything below 'a' fell into the
// uppercase branch and had 32 added to it: "User1" became "user_Q", and "Field_1" became
// "field_\x7fQ", with a DEL control character in a SQL identifier.
//
// For ASCII identifiers the output matches the two regex-based converters -- datasource/sql
// ToSnakeCase, which this package already imports and which DB.Select uses to map result
// columns, and the Cassandra one. Non-ASCII is not covered: those lowercase with unicode rules
// and this loop only touches A-Z, so "MULLER" with an umlaut still keeps its capital.
func toSnakeCase(str string) string {
	diff := 'a' - 'A'
	length := len(str)

	var builder strings.Builder

	// Runs per request, per field: twice per field on a create (bindAndValidateEntity and
	// extractFields) and once on an update, in crud_handlers.go.
	builder.Grow(length + snakeCaseHeadroom)

	for i, char := range str {
		if char < 'A' || char > 'Z' {
			builder.WriteRune(char)
			continue
		}

		// An underscore goes in when the uppercase letter starts a new word: after a lowercase
		// letter or a digit ("user2Name", "x509Cert"), or before a lowercase letter, which is
		// what separates the last capital of an acronym from the word it begins ("userID" stays
		// whole, "IDCard" splits). A digit only counts on the left -- "AB2" is one word.
		prevStartsWord := i > 0 && isLowerOrDigit(rune(str[i-1]))
		nextStartsWord := i < length-1 && isLower(rune(str[i+1]))

		if i != 0 && (prevStartsWord || nextStartsWord) {
			builder.WriteRune('_')
		}

		builder.WriteRune(char + diff)
	}

	return builder.String()
}

// isLower reports whether r is a lowercase ASCII letter. Written out rather than reusing the
// `>= 'a'` shorthand this function used to rely on: that is true for every rune above 'a',
// including every non-ASCII one.
func isLower(r rune) bool { return r >= 'a' && r <= 'z' }

func isLowerOrDigit(r rune) bool { return isLower(r) || (r >= '0' && r <= '9') }
