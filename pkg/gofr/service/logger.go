package service

import (
	"fmt"
	"io"
	"time"
)

type Logger interface {
	Log(args ...any)
}

type Log struct {
	Timestamp     time.Time `json:"timestamp"`
	ResponseTime  int64     `json:"latency"`
	CorrelationID string    `json:"correlationId"`
	ResponseCode  int       `json:"responseCode"`
	HTTPMethod    string    `json:"httpMethod"`
	URI           string    `json:"uri"`
}

func (l *Log) PrettyPrint(writer io.Writer) {
	fmt.Fprintf(writer, "\u001B[38;5;8m%s \u001B[38;5;%dm%-6d\u001B[0m %8d\u001B[38;5;8mµs\u001B[0m %s %s \n",
		l.CorrelationID, colorForStatusCode(l.ResponseCode),
		l.ResponseCode, l.ResponseTime, l.HTTPMethod, l.URI)
}

type ErrorLog struct {
	*Log
	ErrorMessage string `json:"errorMessage"`
}

func (el *ErrorLog) PrettyPrint(writer io.Writer) {
	fmt.Fprintf(writer, "\u001B[38;5;8m%s \u001B[38;5;%dm%-6d\u001B[0m %8d\u001B[38;5;8mµs\u001B[0m %s %s \n",
		el.CorrelationID, colorForStatusCode(el.ResponseCode),
		el.ResponseCode, el.ResponseTime, el.HTTPMethod, el.URI)
}

func colorForStatusCode(status int) int {
	const (
		blue   = 34
		red    = 202
		yellow = 220
	)

	switch {
	case status >= 200 && status < 300:
		return blue
	case status >= 400 && status < 500:
		return yellow
	case status >= 500 && status < 600:
		return red
	}

	return 0
}
