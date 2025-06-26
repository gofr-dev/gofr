package dgraph

import (
	"fmt"
	"regexp"
	"strings"
)

var whitespaceRegex = regexp.MustCompile(`\s+`)

// Logger interface with required methods.
type Logger interface {
	Debug(args ...any)
	Debugf(pattern string, args ...any)
	Log(args ...any)
	Logf(pattern string, args ...any)
	Error(args ...any)
	Errorf(pattern string, args ...any)
}

// QueryLog represents the structure for query logging.
type QueryLog struct {
	Type     string `json:"type"`
	URL      string `json:"url"`
	Duration int64  `json:"duration"` // Duration in microseconds
}

// PrettyPrint logs the QueryLog in a structured format to the given writer.
func (ql *QueryLog) PrettyPrint(logger Logger) {
	// Format the log string
	formattedLog := fmt.Sprintf(
		"\u001B[38;5;8m%-32s \u001B[38;5;206m%-6s\u001B[0m %8d\u001B[38;5;8mµs\u001B[0m %s",
		clean(ql.Type), "DGRAPH", ql.Duration, clean(ql.URL),
	)

	// Log the formatted string using the logger
	logger.Debug(formattedLog)
}

// clean replaces multiple consecutive whitespace characters with a single space and trims leading/trailing whitespace.
func clean(query string) string {
	query = whitespaceRegex.ReplaceAllString(query, " ")
	return strings.TrimSpace(query)
}
