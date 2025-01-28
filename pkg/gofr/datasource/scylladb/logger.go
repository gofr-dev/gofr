package scylladb

import (
	"context"
	"fmt"
	"io"
	"regexp"
	"strings"
)

type Logger interface {
	Debug(args ...any)
	Debugf(pattern string, args ...any)
	Error(args ...any)
	Infof(pattern string, args ...any)
	Errorf(format string, args ...any)
	Log(args ...any)
	Logf(pattern string, args ...any)
}

type QueryLog struct {
	Operation string `json:"operation"`
	Query     string `json:"query"`
	Duration  int64  `json:"duration"`
	Keyspace  string `json:"keyspace,omitempty"`
}

func (ql *QueryLog) PrettyPrint(writer io.Writer) {
	fmt.Fprintf(writer, "\u001B[38;5;8m%-32s \u001B[38;5;206m%-6s\u001B[0m %8d\u001B[38;5;8mµs\u001B[0m %s \u001B[38;5;8m%-32s\u001B[0m\n",
		clean(ql.Operation), "SCYLDB", ql.Duration, clean(ql.Keyspace), clean(ql.Query))
}

var matchSpaces = regexp.MustCompile(`\s+`)

// clean takes a string query as input and performs two operations to clean it up:
// 1. It replaces multiple consecutive whitespace characters with a single space.
// 2. It trims leading and trailing whitespace from the string.
// The cleaned-up query string is then returned.
func clean(query string) string {
	query = matchSpaces.ReplaceAllString(query, " ")
	query = strings.TrimSpace(query)

	return query
}

type Metrics interface {
	NewHistogram(name, desc string, buckets ...float64)
	RecordHistogram(ctx context.Context, name string, value float64, labels ...string)
}
