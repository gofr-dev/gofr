package dgraph

import (
	"fmt"
	"regexp"
	"strings"
)

// Logger interface with required methods
type Logger interface {
	Debug(args ...interface{})
	Debugf(pattern string, args ...interface{})
	Log(args ...interface{})
	Logf(pattern string, args ...interface{})
	Error(args ...interface{})
	Errorf(pattern string, args ...interface{})
}

// QueryLog represents the structure for query logging
type QueryLog struct {
	Type     string `json:"type"`
	URL      string `json:"url"`
	Duration int64  `json:"duration"` // Duration in microseconds
}

// PrettyPrint logs the QueryLog in a structured format to the given writer
func (ql *QueryLog) PrettyPrint(logger Logger) {
	// Format the log string
	formattedLog := fmt.Sprintf(
		"\u001B[38;5;8m%-32s \u001B[38;5;206m%-6s\u001B[0m %8d\u001B[38;5;8mµs\u001B[0m %s",
		clean(ql.URL), "DGRAPH", ql.Duration, clean(ql.Type),
	)

	// Log the formatted string using the logger
	logger.Log(formattedLog)
}

// clean replaces multiple consecutive whitespace characters with a single space and trims leading/trailing whitespace
func clean(query string) string {
	query = regexp.MustCompile(`\s+`).ReplaceAllString(query, " ")
	return strings.TrimSpace(query)
}
