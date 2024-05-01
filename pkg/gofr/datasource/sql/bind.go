package sql

import (
	"fmt"
	"strings"
)

const (
	DialectMysql    = "mysql"
	DialectPostgres = "postgres"
)

const (
	UNKNOWN = iota
	QUESTION
	DOLLAR
)

func Rebind(dialect, query string) string {
	switch bindType(dialect) {
	case QUESTION, UNKNOWN:
		return query
	case DOLLAR:
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

func bindType(dialect string) uint {
	switch dialect {
	case DialectMysql:
		return QUESTION
	case DialectPostgres:
		return DOLLAR
	default:
		return UNKNOWN
	}
}
