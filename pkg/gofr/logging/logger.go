package logging

import (
	"encoding/json"
	"fmt"
	"io"
	"math"
	"os"
	"regexp"
	"strings"
	"time"

	"golang.org/x/term"

	"gofr.dev/pkg/gofr/datasource/redis"
	"gofr.dev/pkg/gofr/datasource/sql"
	"gofr.dev/pkg/gofr/grpc"
	"gofr.dev/pkg/gofr/http/middleware"
	"gofr.dev/pkg/gofr/service"
	"gofr.dev/pkg/gofr/version"
)

const fileMode = 0644

// Logger represents a logging interface.
type Logger interface {
	Debug(args ...interface{})
	Debugf(format string, args ...interface{})
	Log(args ...interface{})
	Logf(format string, args ...interface{})
	Info(args ...interface{})
	Infof(format string, args ...interface{})
	Notice(args ...interface{})
	Noticef(format string, args ...interface{})
	Warn(args ...interface{})
	Warnf(format string, args ...interface{})
	Error(args ...interface{})
	Errorf(format string, args ...interface{})
	Fatal(args ...interface{})
	Fatalf(format string, args ...interface{})
	changeLevel(level Level)
}

type logger struct {
	level      Level
	normalOut  io.Writer
	errorOut   io.Writer
	isTerminal bool
}

type logEntry struct {
	Level       Level       `json:"level"`
	Time        time.Time   `json:"time"`
	Message     interface{} `json:"message"`
	GofrVersion string      `json:"gofrVersion"`
}

func (l *logger) logf(level Level, format string, args ...interface{}) {
	if level < l.level {
		return
	}

	out := l.normalOut
	if level >= ERROR {
		out = l.errorOut
	}

	entry := logEntry{
		Level:       level,
		Time:        time.Now(),
		GofrVersion: version.Framework,
	}

	switch {
	case len(args) == 1 && format == "":
		entry.Message = args[0]
	case len(args) != 1 && format == "":
		entry.Message = args
	case format != "":
		entry.Message = fmt.Sprintf(format+"", args...) // TODO - this is stupid. We should not need empty string.
	}

	if l.isTerminal {
		l.prettyPrint(entry, out)
	} else {
		_ = json.NewEncoder(out).Encode(entry)
	}
}

func (l *logger) Debug(args ...interface{}) {
	l.logf(DEBUG, "", args...)
}

func (l *logger) Debugf(format string, args ...interface{}) {
	l.logf(DEBUG, format, args...)
}

func (l *logger) Info(args ...interface{}) {
	l.logf(INFO, "", args...)
}

func (l *logger) Infof(format string, args ...interface{}) {
	l.logf(INFO, format, args...)
}

func (l *logger) Notice(args ...interface{}) {
	l.logf(NOTICE, "", args...)
}

func (l *logger) Noticef(format string, args ...interface{}) {
	l.logf(NOTICE, format, args...)
}

func (l *logger) Warn(args ...interface{}) {
	l.logf(WARN, "", args...)
}

func (l *logger) Warnf(format string, args ...interface{}) {
	l.logf(WARN, format, args...)
}

func (l *logger) Log(args ...interface{}) {
	l.logf(INFO, "", args...)
}

func (l *logger) Logf(format string, args ...interface{}) {
	l.logf(INFO, format, args...)
}

func (l *logger) Error(args ...interface{}) {
	l.logf(ERROR, "", args...)
}

func (l *logger) Errorf(format string, args ...interface{}) {
	l.logf(ERROR, format, args...)
}

func (l *logger) Fatal(args ...interface{}) {
	l.logf(FATAL, "", args...)

	// exit status is 1 as it denotes failure as signified by Fatal log
	os.Exit(1)
}

func (l *logger) Fatalf(format string, args ...interface{}) {
	l.logf(FATAL, format, args...)
	os.Exit(1)
}

func (l *logger) prettyPrint(e logEntry, out io.Writer) {
	// Giving special treatment to framework's request logs in terminal display. This does not add any overhead
	// in running the server.
	switch msg := e.Message.(type) {
	case middleware.RequestLog:
		fmt.Fprintf(out, "\u001B[38;5;%dm%s\u001B[0m [%s] \u001B[38;5;8m%s \u001B[38;5;%dm%d\u001B[0m "+
			"%8d\u001B[38;5;8mµs\u001B[0m %s %s \n", e.Level.color(), e.Level.String()[0:4],
			e.Time.Format("15:04:05"), msg.TraceID, colorForStatusCode(msg.Response), msg.Response, msg.ResponseTime, msg.Method, msg.URI)
	case sql.Log:
		fmt.Fprintf(out, "\u001B[38;5;%dm%s\u001B[0m [%s] \u001B[38;5;8m%-32s \u001B[38;5;24m%s\u001B[0m %8d\u001B[38;5;8mµs\u001B[0m %s\n",
			e.Level.color(), e.Level.String()[0:4], e.Time.Format("15:04:05"), msg.Type, "SQL", msg.Duration, clean(msg.Query))
	case redis.QueryLog:
		if msg.Query == "pipeline" {
			fmt.Fprintf(out, "\u001B[38;5;%dm%s\u001B[0m [%s] \u001B[38;5;8m%-32s \u001B[38;5;24m%s\u001B[0m %6d\u001B[38;5;8mµs\u001B[0m %s\n",
				e.Level.color(), e.Level.String()[0:4], e.Time.Format("15:04:05"), clean(msg.Query), "REDIS", msg.Duration,
				msg.String()[1:len(msg.String())-1])
		} else {
			fmt.Fprintf(out, "\u001B[38;5;%dm%s\u001B[0m [%s] \u001B[38;5;8m%-32s \u001B[38;5;24m%s\u001B[0m %6d\u001B[38;5;8mµs\u001B[0m %v\n",
				e.Level.color(), e.Level.String()[0:4], e.Time.Format("15:04:05"), clean(msg.Query), "REDIS", msg.Duration, msg.String())
		}
	case service.Log:
		fmt.Fprintf(out, "\u001B[38;5;%dm%s\u001B[0m [%s] \u001B[38;5;8m%s \u001B[38;5;%dm%d\u001B[0m %8d\u001B[38;5;8mµs\u001B[0m %s %s \n",
			e.Level.color(), e.Level.String()[0:4], e.Time.Format("15:04:05"), msg.CorrelationID, colorForStatusCode(msg.ResponseCode),
			msg.ResponseCode, msg.ResponseTime, msg.HTTPMethod, msg.URI)
	case service.ErrorLog:
		fmt.Fprintf(out, "\u001B[38;5;%dm%s\u001B[0m [%s] \u001B[38;5;8m%s "+
			"\u001B[38;5;%dm%d\u001B[0m %8d\u001B[38;5;8mµs\u001B[0m %s %s \033[0;31m %s \n",
			e.Level.color(), e.Level.String()[0:4], e.Time.Format("15:04:05"), msg.CorrelationID, colorForStatusCode(msg.ResponseCode),
			msg.ResponseCode, msg.ResponseTime, msg.HTTPMethod, msg.URI, msg.ErrorMessage)
	case grpc.RPCLog:
		// checking the length of status code to match the spacing that is being done in HTTP logs after status codes
		statusCodeLen := 9 - int(math.Log10(float64(msg.StatusCode))) + 1

		fmt.Fprintf(out, "\u001B[38;5;%dm%s\u001B[0m [%s] \u001B[38;5;8m%s \u001B[38;5;%dm%d"+
			"\u001B[0m %*d\u001B[38;5;8mµs\u001B[0m %s \n",
			e.Level.color(), e.Level.String()[0:4], e.Time.Format("15:04:05"), msg.ID, colorForGRPCCode(msg.StatusCode),
			msg.StatusCode, statusCodeLen, msg.ResponseTime, msg.Method)
	default:
		fmt.Fprintf(out, "\u001B[38;5;%dm%s\u001B[0m [%s] %v\n", e.Level.color(), e.Level.String()[0:4], e.Time.Format("15:04:05"), e.Message)
	}
}

// colorForStatusCode provide color for the status code in the terminal when logs is being pretty-printed.
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

func colorForGRPCCode(status int32) int {
	const (
		blue = 34
		red  = 202
	)

	if status == 0 {
		return blue
	}

	return red
}

// NewLogger creates a new logger instance with the specified logging level.
func NewLogger(level Level) Logger {
	l := &logger{
		normalOut: os.Stdout,
		errorOut:  os.Stderr,
	}

	l.level = level

	l.isTerminal = checkIfTerminal(l.normalOut)

	return l
}

// NewFileLogger creates a new logger instance with logging to a file.
func NewFileLogger(path string) Logger {
	l := &logger{
		normalOut: io.Discard,
		errorOut:  io.Discard,
	}

	if path == "" {
		return l
	}

	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, fileMode)
	if err != nil {
		return l
	}

	l.normalOut = f
	l.errorOut = f

	return l
}

func checkIfTerminal(w io.Writer) bool {
	switch v := w.(type) {
	case *os.File:
		return term.IsTerminal(int(v.Fd()))
	default:
		return false
	}
}

func (l *logger) changeLevel(level Level) {
	l.level = level
}

func clean(query string) string {
	query = regexp.MustCompile(`\s+`).ReplaceAllString(query, " ")
	query = strings.TrimSpace(query)

	return query
}
