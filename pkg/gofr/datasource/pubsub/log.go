package pubsub

import (
	"fmt"
	"io"
)

type Log struct {
	Mode          string `json:"mode"`
	MessageID     string `json:"messageID"`
	MessageValue  string `json:"messageValue"`
	Topic         string `json:"topic"`
	Host          string `json:"host"`
	PubSubBackend string `json:"pubSubBackend"`
}

func (l *Log) PrettyPrint(writer io.Writer) {
	fmt.Fprintf(writer, "\u001B[38;5;8m%-32s \u001B[38;5;24m%s\u001b[0m            %-4s %s \u001b[38;5;101m%s\u001b[0m\n",
		l.MessageID, l.PubSubBackend, l.Mode, l.Topic, l.MessageValue)
}
