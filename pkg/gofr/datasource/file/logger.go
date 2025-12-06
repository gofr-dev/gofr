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

// PrettyPrint formats and prints the log entry to the provided writer with proper column alignment.
func (fl *OperationLog) PrettyPrint(writer io.Writer) {
	operation := cleanString(&fl.Operation)
	provider := fl.Provider
	status := cleanString(fl.Status)
	message := cleanString(fl.Message)

	fmt.Fprintf(writer, "\u001B[38;5;8m%-24s \u001B[38;5;148m%-13s\u001B[0m %8d\u001B[38;5;8mµs\u001B[0m %-10s %s\n",
		operation, provider, fl.Duration, status, message)
}
