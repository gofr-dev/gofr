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

func toSnakeCase(str string) string {
	diff := 'a' - 'A'
	length := len(str)

	var builder strings.Builder

	for i, char := range str {
		if char >= 'a' {
			builder.WriteRune(char)
			continue
		}

		if (i != 0 || i == length-1) && ((i > 0 && rune(str[i-1]) >= 'a') || (i < length-1 && rune(str[i+1]) >= 'a')) {
			builder.WriteRune('_')
		}

		builder.WriteRune(char + diff)
	}

	return builder.String()
}
