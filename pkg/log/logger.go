package log

import (
	"os"
	"sync"
)

// Logger is an interface for level based logging
type Logger interface {
	Log(args ...interface{})
	Logf(format string, a ...interface{})
	Debug(args ...interface{})
	Debugf(format string, a ...interface{})
	Info(args ...interface{})
	Infof(format string, a ...interface{})
	Warn(args ...interface{})
	Warnf(format string, a ...interface{})
	Error(args ...interface{})
	Errorf(format string, a ...interface{})
	Fatal(args ...interface{})
	Fatalf(format string, a ...interface{})
	AddData(key string, value interface{})
}

var (
	rlsInit bool
)

func newLogger() *logger {
	l := &logger{
		out: os.Stdout,
	}

	name := os.Getenv("APP_NAME")
	if name == "" {
		name = "gofr-app"
	}

	version := os.Getenv("APP_VERSION")
	if version == "" {
		version = "dev"
	}

	l.app = appInfo{
		Name:      name,
		Version:   version,
		Framework: "gofr-" + GofrVersion,
		Data:      make(map[string]interface{}),
		syncData:  &sync.Map{},
	}

	l.rls = newLevelService(l, name)

	// Set terminal to ensure proper output format.
	l.isTerminal = checkIfTerminal(l.out)

	return l
}

func NewLogger() Logger {
	return newLogger()
}

func NewCorrelationLogger(correlationID string) Logger {
	l := newLogger()
	l.correlationID = correlationID

	return l
}
