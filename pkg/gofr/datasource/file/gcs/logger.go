package gcs

import (
	"fmt"
	"io"
	"regexp"
	"strings"
)

// FileLog handles logging with different levels.
// In DEBUG MODE, this FileLog can be exported into a file while
// running the application or can be logged in the terminal.
type FileLog struct {
	Operation string  `json:"operation"`
	Duration  int64   `json:"duration"`
	Status    *string `json:"status"`
	Location  string  `json:"location,omitempty"`
	Message   *string `json:"message,omitempty"`
}

var regexpSpaces = regexp.MustCompile(`\s+`)

func clean(query *string) string {
	if query == nil {
		return ""
	}

	return strings.TrimSpace(regexpSpaces.ReplaceAllString(*query, " "))
}

func (fl *FileLog) PrettyPrint(writer io.Writer) {
	fmt.Fprintf(writer, "\u001B[38;5;8m%-32s \u001B[38;5;148m%-6s\u001B[0m %8d\u001B[38;5;8mµs\u001B[0m %-10s \u001B[0m %-48s \n",
		clean(&fl.Operation), "GCS", fl.Duration, clean(fl.Status), clean(fl.Message))
}
