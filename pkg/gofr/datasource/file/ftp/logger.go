package ftp

import (
	"fmt"
	"io"
	"regexp"
	"strings"
)

// FileLog handles logging with different levels.
type FileLog struct {
	Operation string `json:"operation"`
	Duration  int64  `json:"duration"`
	Location  string `json:"filepath,omitempty"`
	Message   string `json:"message,omitempty"`
}

var regexpSpaces = regexp.MustCompile(`\s+`)

func clean(query string) string {
	return strings.TrimSpace(regexpSpaces.ReplaceAllString(query, " "))
}

func (fl *FileLog) PrettyPrint(writer io.Writer) {
	fmt.Fprintf(writer, "\u001B[38;5;8m%-32s \u001B[38;5;206m%-6s\u001B[0m %8s\u001B[38;5;8mµs\u001B[0m %s\n",
		clean(fl.Operation), "FTP", fmt.Sprint(fl.Duration),
		clean(fl.Message))
}
