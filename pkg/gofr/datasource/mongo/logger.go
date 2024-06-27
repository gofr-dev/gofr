package mongo

import (
	"fmt"
	"io"
	"regexp"
	"strings"
)

type Logger interface {
	Debugf(pattern string, args ...interface{})
	Logf(pattern string, args ...interface{})
	Errorf(pattern string, args ...interface{})
}

type QueryLog struct {
	Query      string      `json:"query"`
	Duration   int64       `json:"duration"`
	Collection string      `json:"collection,omitempty"`
	Filter     interface{} `json:"filter,omitempty"`
	ID         interface{} `json:"id,omitempty"`
	Update     interface{} `json:"update,omitempty"`
}

func (ql *QueryLog) PrettyPrint(writer io.Writer) {
	if ql.Filter == nil {
		ql.Filter = ""
	}

	if ql.ID == nil {
		ql.ID = ""
	}

	if ql.Update == nil {
		ql.Update = ""
	}

	fmt.Fprintf(writer, "\u001B[38;5;8m%-32s \u001B[38;5;206m%-6s\u001B[0m %8d\u001B[38;5;8mµs\u001B[0m %s\n",
		clean(ql.Query), "MONGO", ql.Duration,
		clean(strings.Join([]string{ql.Collection, fmt.Sprint(ql.Filter), fmt.Sprint(ql.ID), fmt.Sprint(ql.Update)}, " ")))
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
