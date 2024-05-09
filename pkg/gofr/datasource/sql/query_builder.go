package sql

import (
	"fmt"
	"strings"
)

func InsertQuery(dialect, tableName string, fieldNames []string) string {
	fieldNamesLength := len(fieldNames)

	var bindVars []string
	for i := 1; i <= fieldNamesLength; i++ {
		bindVars = append(bindVars, bindVar(dialect, i))
	}

	q := quote(dialect)

	stmt := fmt.Sprintf(`INSERT INTO %s (%s) VALUES (%s)`,
		quotedString(q, tableName),
		quotedString(q, strings.Join(fieldNames, quotedString(q, ", "))),
		strings.Join(bindVars, ", "),
	)

	return stmt
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
