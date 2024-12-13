package sql

import (
	"fmt"
)

const (
	quoteBack   = "`"
	quoteDouble = `"`
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
	case DialectMySQL:
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
		return quoteDouble
	}

	return quoteBack
}

func quotedString(q, s string) string {
	return fmt.Sprintf("%s%s%s", q, s, q)
}
