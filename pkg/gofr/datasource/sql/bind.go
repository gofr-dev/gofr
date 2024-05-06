package sql

import (
	"fmt"
)

const (
	DialectMysql    = "mysql"
	DialectPostgres = "postgres"

	QuoteBack   = "`"
	QuoteDouble = `"`
)

// BindVarType represents different type of bindvars in SQL queries.
type BindVarType uint

const (
	UNKNOWN BindVarType = iota + 1
	QUESTION
	DOLLAR
)

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

func bindVar(dialect string, position int) string {
	if DOLLAR == bindType(dialect) {
		return fmt.Sprintf("$%v", position)
	}

	return "?"
}
func quote(dialect string) string {
	if DialectPostgres == dialect {
		return QuoteDouble
	}

	return QuoteBack
}

func quotedString(q, s string) string {
	return fmt.Sprintf("%s%s%s", q, s, q)
}
