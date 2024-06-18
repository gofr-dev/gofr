package clickhouse

import (
	"fmt"
	"io"
	"regexp"
	"strings"
)

type Logger interface {
	Debugf(pattern string, args ...interface{})
	Debug(args ...interface{})
	Logf(pattern string, args ...interface{})
	Errorf(patter string, args ...interface{})
}

type Log struct {
	Type     string        `json:"type"`
	Query    string        `json:"query"`
	Duration int64         `json:"duration"`
	Args     []interface{} `json:"args,omitempty"`
}

func (l *Log) PrettyPrint(writer io.Writer) {
	fmt.Fprintf(writer, "\u001B[38;5;8m%-32s \u001B[38;5;24m%-6s\u001B[0m %8d\u001B[38;5;8mµs\u001B[0m %s\n",
		l.Type, "CHDB", l.Duration, clean(l.Query))
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
