package badger

import (
	"fmt"
	"io"
	"strings"
)

type Logger interface {
	Debug(args ...any)
	Debugf(pattern string, args ...any)
	Info(args ...any)
	Infof(pattern string, args ...any)
	Error(args ...any)
	Errorf(patter string, args ...any)
}

type Log struct {
	Type     string `json:"type"`
	Duration int64  `json:"duration"`
	Key      string `json:"key"`
	Value    string `json:"value,omitempty"`
}

func (l *Log) PrettyPrint(writer io.Writer) {
	fmt.Fprintf(writer, "\u001B[38;5;8m%-32s \u001B[38;5;162m%-6s\u001B[0m %8d\u001B[38;5;8mµs\u001B[0m %s \n",
		l.Type, "BADGR", l.Duration, strings.Join([]string{l.Key, l.Value}, " "))
}
