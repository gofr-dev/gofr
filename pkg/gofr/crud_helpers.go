package gofr

import (
	"fmt"
	"reflect"
	"strconv"
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

func convertIDType(id string, fieldType reflect.Type) (any, error) {
	val := reflect.New(fieldType).Elem()

	switch fieldType.Kind() {
	case reflect.String:
		val.SetString(id)
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		intVal, err := strconv.ParseInt(id, 10, fieldType.Bits())
		if err != nil {
			return nil, err
		}
		val.SetInt(intVal)
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		uintVal, err := strconv.ParseUint(id, 10, fieldType.Bits())
		if err != nil {
			return nil, err
		}
		val.SetUint(uintVal)
	default:
		return nil, fmt.Errorf("unsupported ID type: %s", fieldType.Kind())
	}

	return val.Interface(), nil
}
