package oracle

import (
	"fmt"
	"io"
	"regexp"
	"strings"
)

type Logger interface {
	Debugf(pattern string, args ...any)
	Debug(args ...any)
	Logf(pattern string, args ...any)
	Errorf(pattern string, args ...any)
}

type Log struct {
	Type     string `json:"type"`
	Query    string `json:"query"`
	Duration int64  `json:"duration"`
	Args     []any  `json:"args,omitempty"`
}

func (l *Log) PrettyPrint(writer io.Writer) {
	fmt.Fprintf(writer, "%-10s ORACLE %8dµs %s\n", l.Type, l.Duration, clean(l.Query))
}

func clean(query string) string {
	query = regexp.MustCompile(`\s+`).ReplaceAllString(query, " ")
	return strings.TrimSpace(query)
}
