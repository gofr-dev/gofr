package sql

import (
	"errors"
	"fmt"
	"reflect"
	"strings"
)

var (
	errFieldCannotBeEmpty = errors.New("field cannot be empty")
	errFieldCannotBeZero  = errors.New("field cannot be zero")
	errFieldCannotBeNull  = errors.New("field cannot be null")
)

type FieldConstraints struct {
	AutoIncrement bool
	NotNull       bool
}

func InsertQuery(dialect, tableName string, fieldNames []string, values []any,
	constraints map[string]FieldConstraints) (string, error) {
	bindVars := make([]string, 0, len(fieldNames))
	columns := make([]string, 0, len(fieldNames))

	for i, fieldName := range fieldNames {
		if constraints[fieldName].AutoIncrement {
			continue
		}

		if err := validateNotNull(fieldName, values[i], constraints[fieldName].NotNull); err != nil {
			return "", err
		}

		bindVars = append(bindVars, bindVar(dialect, i+1))
		columns = append(columns, quotedString(quote(dialect), fieldName))
	}

	q := quote(dialect)

	stmt := fmt.Sprintf(`INSERT INTO %s (%s) VALUES (%s)`,
		quotedString(q, tableName),
		strings.Join(columns, ", "),
		strings.Join(bindVars, ", "),
	)

	return stmt, nil
}

func SelectQuery(dialect, tableName string) string {
	return fmt.Sprintf(`SELECT * FROM %s`, quotedString(quote(dialect), tableName))
}

func SelectByQuery(dialect, tableName, field string) string {
	q := quote(dialect)

	return fmt.Sprintf(`SELECT * FROM %s WHERE %s=%s`,
		quotedString(q, tableName),
		quotedString(q, field),
		bindVar(dialect, 1))
}

func UpdateByQuery(dialect, tableName string, fieldNames []string, field string) string {
	q := quote(dialect)
	fieldNamesLength := len(fieldNames)

	var paramsList []string
	for i := 0; i < fieldNamesLength; i++ {
		paramsList = append(paramsList, fmt.Sprintf(`%s=%s`, quotedString(q, fieldNames[i]), bindVar(dialect, i+1)))
	}

	stmt := fmt.Sprintf(`UPDATE %s SET %s WHERE %s=%s`,
		quotedString(q, tableName),
		strings.Join(paramsList, ", "),
		quotedString(q, field),
		bindVar(dialect, fieldNamesLength+1),
	)

	return stmt
}

func DeleteByQuery(dialect, tableName, field string) string {
	q := quote(dialect)

	return fmt.Sprintf(`DELETE FROM %s WHERE %s=%s`,
		quotedString(q, tableName),
		quotedString(q, field),
		bindVar(dialect, 1))
}

// validateNotNull rejects a value that cannot satisfy a NOT NULL column.
//
// Dispatch is on reflect.Kind rather than on the concrete type. A type switch only matches the
// predeclared types exactly, so a named type -- `type Count uint`, which is ordinary in an entity
// struct -- misses every case and lands in the default path. On the kind, Count and uint are the
// same thing and get the same answer.
//
//nolint:exhaustive // every remaining kind is deliberately handled by validateDefaultNotNull.
func validateNotNull(fieldName string, value any, isNotNull bool) error {
	if !isNotNull {
		return nil
	}

	switch reflect.ValueOf(value).Kind() {
	case reflect.String:
		return validateStringNotNull(fieldName, value)
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return validateIntNotNull(fieldName, value)
	// reflect.Value.Int panics on unsigned kinds, so these read through Uint instead.
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		return validateUintNotNull(fieldName, value)
	case reflect.Float32, reflect.Float64:
		return validateFloatNotNull(fieldName, value)
	default:
		return validateDefaultNotNull(fieldName, value)
	}
}

func validateStringNotNull(fieldName string, value any) error {
	if reflect.ValueOf(value).String() == "" {
		return fmt.Errorf("%w: %s", errFieldCannotBeEmpty, fieldName)
	}

	return nil
}

func validateIntNotNull(fieldName string, value any) error {
	if reflect.ValueOf(value).Int() == 0 {
		return fmt.Errorf("%w: %s", errFieldCannotBeZero, fieldName)
	}

	return nil
}

// validateUintNotNull handles the unsigned integer kinds. reflect.Value.Int()
// panics for unsigned values, so these must use reflect.Value.Uint().
func validateUintNotNull(fieldName string, value any) error {
	if reflect.ValueOf(value).Uint() == 0 {
		return fmt.Errorf("%w: %s", errFieldCannotBeZero, fieldName)
	}

	return nil
}

func validateFloatNotNull(fieldName string, value any) error {
	if reflect.ValueOf(value).Float() == 0.0 {
		return fmt.Errorf("%w: %s", errFieldCannotBeZero, fieldName)
	}

	return nil
}

func validateDefaultNotNull(fieldName string, value any) error {
	v := reflect.ValueOf(value)

	// An untyped-nil interface has no underlying value at all, which for a
	// NOT NULL column is a null.
	if v.Kind() == reflect.Invalid {
		return fmt.Errorf("%w: %s", errFieldCannotBeNull, fieldName)
	}

	// reflect.Value.IsNil panics on non-nillable kinds (bool, struct such as
	// time.Time, array, uintptr, complex, ...). Those can never be nil, so the
	// nil check only runs for the kinds where IsNil is defined.
	if isNillableKind(v.Kind()) && v.IsNil() {
		return fmt.Errorf("%w: %s", errFieldCannotBeNull, fieldName)
	}

	return nil
}

// isNillableKind reports whether reflect.Value.IsNil is defined for k. Calling
// IsNil on any other kind panics.
//
//nolint:exhaustive // every non-nillable kind is intentionally handled by the default.
func isNillableKind(k reflect.Kind) bool {
	switch k {
	case reflect.Chan, reflect.Func, reflect.Interface, reflect.Map, reflect.Pointer, reflect.Slice:
		return true
	default:
		return false
	}
}
