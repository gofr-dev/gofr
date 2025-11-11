package dbresolver

import (
	"fmt"
	"regexp"
	"strings"
)

var whitespaceRegex = regexp.MustCompile(`\s+`)

// Logger defines the logging interface for dbresolver.
type Logger interface {
	Debug(args ...any)
	Debugf(pattern string, args ...any)
	Logf(pattern string, args ...any)
	Errorf(pattern string, args ...any)
	Warnf(pattern string, args ...any)
	Info(args ...any)
	Infof(format string, args ...any)
	Error(args ...any)
	Warn(args ...any)
}

// QueryLog contains information about a SQL query.
type QueryLog struct {
	Type     string `json:"type"`
	Query    string `json:"query"`
	Duration int64  `json:"duration"`
	Target   string `json:"target"`
	IsRead   bool   `json:"is_read"`
}

// PrettyPrint formats the QueryLog for output.
func (ql *QueryLog) PrettyPrint(logger Logger) {
	formattedLog := fmt.Sprintf("\u001B[38;5;8m%-32s \u001B[38;5;206m%-6s\u001B[0m %8d\u001B[38;5;8mµs\u001B[0m %s %s\n",
		clean(ql.Type), "DBRESOLVER", ql.Duration,
		clean(fmt.Sprintf("%s %s", ql.Target, ql.Query)), clean(ql.Query))

	logger.Debug(formattedLog)
}

func clean(query string) string {
	// Replace multiple consecutive whitespace characters with a single space
	query = whitespaceRegex.ReplaceAllString(query, " ")
	// Trim leading and trailing whitespace from the string
	return strings.TrimSpace(query)
}
