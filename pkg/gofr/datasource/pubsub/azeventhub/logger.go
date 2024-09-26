package azeventhub

import (
	"fmt"
	"io"
)

// Logger interface with required methods
type Logger interface {
	Debug(args ...interface{})
	Debugf(pattern string, args ...interface{})
	Log(args ...interface{})
	Logf(pattern string, args ...interface{})
	Error(args ...interface{})
	Fatal(args ...interface{})
	Errorf(pattern string, args ...interface{})
}

type Log struct {
	Mode          string `json:"mode"`
	MessageValue  string `json:"messageValue"`
	Topic         string `json:"topic"`
	Host          string `json:"host"`
	PubSubBackend string `json:"pubSubBackend"`
	Time          int64  `json:"time"`
}

func (l *Log) PrettyPrint(writer io.Writer) {
	fmt.Fprintf(writer, "\u001B[38;5;8m%-32s \u001B[38;5;24m%-6s\u001B[0m %8d\u001B[38;5;8mµs\u001B[0m %-4s%s \u001b[38;5;101m\n",
		l.Topic, l.PubSubBackend, l.Time, l.Mode, l.MessageValue)
}
