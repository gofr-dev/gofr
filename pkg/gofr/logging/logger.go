package logging

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"time"

	"gofr.dev/pkg/gofr/config"

	"golang.org/x/term"

	"gofr.dev/pkg/gofr/datasource/redis"
	"gofr.dev/pkg/gofr/datasource/sql"
	"gofr.dev/pkg/gofr/http/middleware"
	"gofr.dev/pkg/gofr/service"
)

type Logger interface {
	Debug(args ...interface{})
	Debugf(format string, args ...interface{})
	Log(args ...interface{})
	Logf(format string, args ...interface{})
	Info(args ...interface{})
	Infof(format string, args ...interface{})
	Error(args ...interface{})
	Errorf(format string, args ...interface{})
}

type logger struct {
	level      Level
	normalOut  io.Writer
	errorOut   io.Writer
	isTerminal bool
}

type logEntry struct {
	Level   Level       `json:"Level"`
	Time    time.Time   `json:"time"`
	Message interface{} `json:"message"`
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
		Level: level,
		Time:  time.Now(),
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

func (l *logger) prettyPrint(e logEntry, out io.Writer) {
	// Giving special treatment to framework's request logs in terminal display. This does not add any overhead
	// in running the server.
	switch msg := e.Message.(type) {
	case middleware.RequestLog:
		fmt.Fprintf(out, "\u001B[38;5;%dm%s\u001B[0m [%s] \u001B[38;5;8m%s \u001B[38;5;%dm%d\u001B[0m "+
			"%8d\u001B[38;5;8mµs\u001B[0m %s %s \n", e.Level.color(), e.Level.String()[0:4],
			e.Time.Format("15:04:05"), msg.ID, colorForStatusCode(msg.Response), msg.Response, msg.ResponseTime, msg.Method, msg.URI)
	case sql.Log:
		fmt.Fprintf(out, "\u001B[38;5;%dm%s\u001B[0m [%s] \u001B[38;5;8m%-32s \u001B[38;5;24m%s\u001B[0m %8d\u001B[38;5;8mµs\u001B[0m %s\n",
			e.Level.color(), e.Level.String()[0:4], e.Time.Format("15:04:05"), msg.Type, "SQL", msg.Duration, msg.Query)
	case redis.QueryLog:
		if msg.Query == "pipeline" {
			fmt.Fprintf(out, "\u001B[38;5;%dm%s\u001B[0m [%s] \u001B[38;5;8m%-32s \u001B[38;5;24m%s\u001B[0m %6d\u001B[38;5;8mµs\u001B[0m %s\n",
				e.Level.color(), e.Level.String()[0:4], e.Time.Format("15:04:05"), msg.Query, "REDIS", msg.Duration,
				msg.String()[1:len(msg.String())-1])
		} else {
			fmt.Fprintf(out, "\u001B[38;5;%dm%s\u001B[0m [%s] \u001B[38;5;8m%-32s \u001B[38;5;24m%s\u001B[0m %6d\u001B[38;5;8mµs\u001B[0m %v\n",
				e.Level.color(), e.Level.String()[0:4], e.Time.Format("15:04:05"), msg.Query, "REDIS", msg.Duration, msg.String())
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

func NewLogger(conf config.Config) Logger {
	l := &logger{
		normalOut: os.Stdout,
		errorOut:  os.Stderr,
	}

	l.level = GetLevelFromString(conf.Get("LOG_LEVEL"))

	l.isTerminal = checkIfTerminal(l.normalOut)

	if url := conf.Get("REMOTE_LOG_URL"); url != "" {
		appName := conf.Get("APP_NAME")
		accessKey := conf.Get("REMOTE_ACCESS_KEY")

		remoteLogger := &RemoteLevelService{
			url:       url,
			accessKey: accessKey,
			appName:   appName,
			LogLevel:  l.level,
			logger:    l,
			ticker:    time.NewTicker(levelFetchInterval * time.Second),
		}

		go remoteLogger.updateLogLevel()
	}

	return l
}

// TODO - Do we need this? Only used for CMD log silencing.
func NewSilentLogger() Logger {
	l := &logger{
		normalOut: io.Discard,
		errorOut:  io.Discard,
	}

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
