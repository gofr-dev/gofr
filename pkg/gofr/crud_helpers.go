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

// toSnakeCase converts a Go identifier to the snake_case name used for a table or column.
//
// Only uppercase ASCII letters are special: they are lowercased, and separated from what comes
// before with an underscore. Every other rune -- digits, underscores, punctuation, non-ASCII --
// passes through untouched. The rule is stated positively on purpose. The earlier guard was
// `char >= 'a'`, which relies on an ordering accident, and everything below 'a' fell into the
// uppercase branch and had 32 added to it: "User1" became "user_Q", and "Field_1" became
// "field_\x7fQ", with a DEL control character in a SQL identifier.
//
// Output matches the regex-based toSnakeCase in the Cassandra datasource for every identifier
// shape tested, so an entity maps to the same column name on either backend.
func toSnakeCase(str string) string {
	diff := 'a' - 'A'
	length := len(str)

	var builder strings.Builder

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

		if (i != 0 || i == length-1) && (prevStartsWord || nextStartsWord) {
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
