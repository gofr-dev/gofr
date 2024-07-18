package badger

import (
	"fmt"
	"io"
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
}

func (l *Log) PrettyPrint(writer io.Writer) {
	fmt.Fprintf(writer, "\u001B[38;5;8m%-32s \u001B[38;5;206m%-6s\u001B[0m %8d\u001B[38;5;8mµs\u001B[0m\n",
		l.Type, "BADG", l.Duration)
}
