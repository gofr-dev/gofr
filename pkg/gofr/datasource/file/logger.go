package file

import (
	"fmt"
	"io"
	"regexp"
	"strings"
)

// OperationLog represents a standardized log entry for file operations.
type OperationLog struct {
	Operation string  `json:"operation"`
	Duration  int64   `json:"duration"`
	Status    *string `json:"status"`
	Location  string  `json:"location,omitempty"`
	Message   *string `json:"message,omitempty"`
	Provider  string  `json:"provider"` // Identifies the storage provider
}

var regexpSpaces = regexp.MustCompile(`\s+`)

// cleanString standardizes string formatting for logs/metrics.
func cleanString(query *string) string {
	if query == nil {
		return ""
	}

	return strings.TrimSpace(regexpSpaces.ReplaceAllString(*query, " "))
}

// PrettyPrint formats and prints the log entry to the provided writer.
func (fl *OperationLog) PrettyPrint(writer io.Writer) {
	fmt.Fprintf(writer, "\u001B[38;5;8m%-32s \u001B[38;5;148m%-6s\u001B[0m %8d\u001B[38;5;8mµs\u001B[0m %-10s \u001B[0m %-48s \n",
		cleanString(&fl.Operation), fl.Provider, fl.Duration, cleanString(fl.Status), cleanString(fl.Message))
}
