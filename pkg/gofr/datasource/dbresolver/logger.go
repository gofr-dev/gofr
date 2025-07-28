package dbresolver

import (
	"fmt"
	"io"
	"regexp"
	"strings"
)

// Logger defines the logging interface for dbresolver
type Logger interface {
	Debug(args ...any)
	Debugf(pattern string, args ...any)
	Logf(pattern string, args ...any)
	Errorf(pattern string, args ...any)
	Warnf(pattern string, args ...any)
}

// QueryLog contains information about a SQL query
type QueryLog struct {
	Query     string `json:"query"`
	Duration  int64  `json:"duration"`
	Operation string `json:"operation"`
	Target    string `json:"target"`
	QueryType string `json:"queryType"`
}

// PrettyPrint formats the QueryLog for output
func (ql *QueryLog) PrettyPrint(writer io.Writer) {
	fmt.Fprintf(writer, "\u001B[38;5;8m%-32s \u001B[38;5;206m%-6s\u001B[0m %8d\u001B[38;5;8mµs\u001B[0m %s %s\n",
		clean(ql.Operation), "SQLROUTER", ql.Duration,
		clean(fmt.Sprintf("%s %s", ql.Target, ql.QueryType)), clean(ql.Query))
}

func clean(query string) string {
	// Replace multiple consecutive whitespace characters with a single space
	query = regexp.MustCompile(`\s+`).ReplaceAllString(query, " ")
	// Trim leading and trailing whitespace from the string
	return strings.TrimSpace(query)
}
