package scylladb

import (
	"fmt"
	"io"
	"regexp"
	"strings"
)

type Logger interface {
	Debug(args ...interface{})
	Debugf(pattern string, args ...interface{})
	Error(args ...interface{})
	Infof(pattern string, args ...interface{})
	Errorf(format string, args ...interface{})
	Log(args ...interface{})
	Logf(pattern string, args ...interface{})
}

type QueryLog struct {
	Operation string `json:"operation"`
	Query     string `json:"query"`
	Duration  int64  `json:"duration"`
	Keyspace  string `json:"keyspace,omitempty"`
}

func (ql *QueryLog) PrettyPrint(writer io.Writer) {
	fmt.Fprintf(writer, "\u001B[38;5;8m%-32s \u001B[38;5;206m%-6s\u001B[0m %8d\u001B[38;5;8mµs\u001B[0m %s \u001B[38;5;8m%-32s\u001B[0m\n",
		clean(ql.Operation), "CASS", ql.Duration, clean(ql.Keyspace), clean(ql.Query))
}

func clean(query string) string {

	query = regexp.MustCompile(`\s+`).ReplaceAllString(query, " ")

	query = strings.TrimSpace(query)

	return query
}
