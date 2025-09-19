package influxdb

import (
	"fmt"
	"io"
	"regexp"
	"strings"
)

type Logger interface {
	Debug(args ...any)
	Debugf(pattern string, args ...any)
	Log(args ...any)
	Logf(pattern string, args ...any)
	Error(args ...any)
	Errorf(pattern string, args ...any)
}

type QueryLog struct {
	Operation string `json:"operation"`
	Query     string `json:"query"`
	Duration  int64  `json:"duration"`
	Keyspace  string `json:"keyspace,omitempty"`
	Args      []any  `json:"args,omitempty"`
}

func (ql *QueryLog) PrettyPrint(writer io.Writer) {
	var argsStr string

	if len(ql.Args) > 0 {
		var parts []string

		for _, a := range ql.Args {
			parts = append(parts, clean(fmt.Sprintf("%v", a)))
		}

		argsStr = " [" + strings.Join(parts, ", ") + "]"
	}

	fmt.Fprintf(
		writer,
		"\u001B[38;5;8m%-32s \u001B[38;5;206m%-6s\u001B[0m %8d\u001B[38;5;8mms\u001B[0m %s \u001B[38;5;8m%-32s\u001B[0m%s\n",
		clean(ql.Operation),
		"INFL",
		ql.Duration,
		clean(ql.Keyspace),
		clean(ql.Query),
		argsStr,
	)
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
