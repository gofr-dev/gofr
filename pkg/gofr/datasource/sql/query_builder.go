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

func validateNotNull(fieldName string, value any, isNotNull bool) error {
	if !isNotNull {
		return nil
	}

	switch v := value.(type) {
	case string:
		return validateStringNotNull(fieldName, v)
	case int, int8, int16, int32, int64, uint, uint8, uint16, uint32, uint64:
		return validateIntNotNull(fieldName, v)
	case float32, float64:
		return validateFloatNotNull(fieldName, v)
	default:
		return validateDefaultNotNull(fieldName, value)
	}
}

func validateStringNotNull(fieldName, value string) error {
	if value == "" {
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

func validateFloatNotNull(fieldName string, value any) error {
	if reflect.ValueOf(value).Float() == 0.0 {
		return fmt.Errorf("%w: %s", errFieldCannotBeZero, fieldName)
	}

	return nil
}

func validateDefaultNotNull(fieldName string, value any) error {
	if reflect.ValueOf(value).IsNil() {
		return fmt.Errorf("%w: %s", errFieldCannotBeNull, fieldName)
	}

	return nil
}
