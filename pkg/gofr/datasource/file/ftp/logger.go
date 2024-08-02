package ftp

import (
	"fmt"
	"io"
	"regexp"
	"strings"
)

// FileLog handles logging with different levels.
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
	fmt.Fprintf(writer, "\u001B[38;5;8m%-20s \u001B[38;5;206m%-6s \u001B[0m %-8s \u001B[38;5;206m%s \u001B[38;5;8mµs\u001B[0m %-48s\u001B[0m %s\n",
		clean(&fl.Operation), "FTP", clean(fl.Status), fmt.Sprint(fl.Duration),
		clean(&fl.Location), clean(fl.Message))
}
