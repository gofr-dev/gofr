package couchbase

import (
	"fmt"
	"io"
	"regexp"
	"strings"
)

// Logger is an interface for logging messages.
type Logger interface {
	Debug(args ...any)
	Debugf(pattern string, args ...any)
	Log(args ...any)
	Logf(pattern string, args ...any)
	Error(args ...any)
	Errorf(pattern string, args ...any)
}

// QueryLog represents a log entry for a Couchbase query.
type QueryLog struct {
	Query      string `json:"query"`
	Duration   int64  `json:"duration"`
	Key        string `json:"key,omitempty"`
	Statement  string `json:"statement,omitempty"`
	Parameters any    `json:"parameters,omitempty"`
}

// PrettyPrint prints the query log in a human-readable format.
func (ql *QueryLog) PrettyPrint(writer io.Writer) {
	if ql.Key == "" && ql.Statement == "" {
		return
	}

	if ql.Parameters == nil {
		ql.Parameters = ""
	}

	query := ql.Query
	if query == "" {
		query = ql.Statement
	}

	fmt.Fprintf(writer, "\u001B[38;5;8m%-32s \u001B[38;5;207m%-6s\u001B[0m %8d\u001B[38;5;8m\b 5s\u001B[0m %s\n",
		clean(query), "COUCHBASE", ql.Duration,
		clean(strings.Join([]string{ql.Key + " " + fmt.Sprint(ql.Parameters)}, " ")))
}

// clean takes a string query as input and performs two operations to clean it up:
// 1. It replaces multiple consecutive whitespace characters with a single space.
// 2. It trims leading and trailing whitespace from the string.
// The cleaned-up query string is then returned.
func clean(query string) string {
	// Replace multiple consecutive whitespace characters with a single space
	query = regexp.MustCompile(`\s+`).ReplaceAllString(query, " ")

	// Trim leading and trailing whitespace from the string
	query = strings.TrimSpace(query)

	return query
}
