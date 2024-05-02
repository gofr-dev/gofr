package sql

import (
	"fmt"
	"strings"
)

const (
	DialectMysql    = "mysql"
	DialectPostgres = "postgres"
)

// BindVarType represents different type of bindvars in SQL queries.
type BindVarType uint

const (
	UNKNOWN BindVarType = iota + 1
	QUESTION
	DOLLAR
)

func Rebind(dialect, query string) string {
	if DOLLAR == bindType(dialect) {
		queryFormat := strings.Replace(query, "?", "%v", -1)
		count := strings.Count(query, "?")
		replacement := make([]interface{}, count)

		for i := 0; i < count; i++ {
			replacement[i] = fmt.Sprintf("$%v", i+1)
		}

		return fmt.Sprintf(queryFormat, replacement...)
	}

	return query
}

func bindType(dialect string) BindVarType {
	switch dialect {
	case DialectMysql:
		return QUESTION
	case DialectPostgres:
		return DOLLAR
	default:
		return UNKNOWN
	}
}
