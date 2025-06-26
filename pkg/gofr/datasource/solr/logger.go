package solr

import (
	"fmt"
	"io"
	"regexp"
	"strings"
)

type Logger interface {
	Debug(args ...any)
	Debugf(pattern string, args ...any)
	Info(args ...any)
	Infof(pattern string, args ...any)
	Error(args ...any)
	Errorf(pattern string, args ...any)
}

type QueryLog struct {
	Type     string `json:"type"`
	URL      string `json:"Url"`
	Duration int64  `json:"duration"`
}

func (ql *QueryLog) PrettyPrint(writer io.Writer) {
	fmt.Fprintf(writer, "\u001B[38;5;8m%-32s \u001B[38;5;206m%-6s\u001B[0m %8d\u001B[38;5;8mµs\u001B[0m %s\n",
		clean(ql.URL), "SOLR", ql.Duration, clean(ql.Type))
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
