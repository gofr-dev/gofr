package arangodb

import (
	"fmt"
	"io"
	"regexp"
	"strings"
)

type Logger interface {
	Debug(args ...any)
	Debugf(pattern string, args ...any)
	Logf(pattern string, args ...any)
	Errorf(pattern string, args ...any)
}

type QueryLog struct {
	Query      string `json:"query"`
	Duration   int64  `json:"duration"`
	Database   string `json:"database,omitempty"`
	Collection string `json:"collection,omitempty"`
	Filter     any    `json:"filter,omitempty"`
	ID         any    `json:"id,omitempty"`
	Operation  string `json:"operation,omitempty"`
}

// PrettyPrint formats the QueryLog for output.
func (ql *QueryLog) PrettyPrint(writer io.Writer) {
	if ql.Filter == nil {
		ql.Filter = ""
	}

	if ql.ID == nil {
		ql.ID = ""
	}

	fmt.Fprintf(writer, "\u001B[38;5;8m%-32s \u001B[38;5;206m%-6s\u001B[0m %8d\u001B[38;5;8mµs\u001B[0m %s %s\n",
		clean(ql.Operation), "ARANGODB", ql.Duration,
		clean(strings.Join([]string{ql.Database, ql.Collection, fmt.Sprint(ql.Filter), fmt.Sprint(ql.ID)}, " ")), clean(ql.Query))
}

func clean(query string) string {
	// Replace multiple consecutive whitespace characters with a single space
	query = regexp.MustCompile(`\s+`).ReplaceAllString(query, " ")
	// Trim leading and trailing whitespace from the string
	return strings.TrimSpace(query)
}
