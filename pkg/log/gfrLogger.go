/*
Package log provides  logger for logging various output messages in different formats, including color-coded terminal
output and JSON.Gofr logger supports different log levels such as INFO, ERROR, WARN, DEBUG, or FATAL.
*/
package log

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"runtime/debug"
	"strconv"
	"sync"
)

type logger struct {
	out io.Writer

	// App Specific Data for the logger
	app           appInfo
	correlationID string

	isTerminal bool

	// remote logging service
	rls *levelService
}

type appInfo struct {
	Name      string                 `json:"name"`
	Version   string                 `json:"version"`
	Framework string                 `json:"framework"`
	Data      map[string]interface{} `json:"data,omitempty"`
	syncData  *sync.Map
}

func (a *appInfo) getAppData() appInfo {
	res := appInfo{}

	res.Name = a.Name
	res.Version = a.Version
	res.Framework = a.Framework
	res.Data = make(map[string]interface{})

	if a.syncData != nil {
		a.syncData.Range(func(key, value interface{}) bool {
			res.Data[key.(string)] = value
			return true
		})
	}

	return res
}

// log does the actual logging. This function creates the entry message and outputs it in color format
// in terminal context and gives out json in non terminal context. Also, sends to echo if client is present.
func (l *logger) log(level level, format string, args ...interface{}) {
	lvl := l.rls.level
	if lvl < level {
		return // No need to do anything if we are not going to log it.
	}

	e, data, isPerformanceLog := entryFromInputs(format, args...)

	e.Level = level
	e.System = fetchSystemStats()
	e.App = l.app.getAppData()

	if isPerformanceLog {
		// in performance log, app data is under the key `appData` instead of e.App.Data.
		// This is done to ensure that appData is consistent for concurrent requests
		appData, _ := e.Data["appData"].(map[string]interface{})
		e.App.Data = appData
		delete(e.Data, "appData")
	}

	// logging struct/map in app data
	for key, val := range data {
		e.App.Data[key] = val
	}

	if l.correlationID != "" { // CorrelationID from Application Log
		e.CorrelationID = l.correlationID
	} else if correlationID, ok := e.App.Data["correlationID"]; ok {
		/*CorrelationID for middleware apart from Performance log.
		For performance log the correlationID comes from Logline struct defined in logging.go.*/
		e.CorrelationID, _ = correlationID.(string)
	}

	id, err := strconv.Atoi(e.CorrelationID)
	if err == nil && id == 0 {
		e.CorrelationID = ""
	}

	// Deleting the correlationId in case of any duplication.
	delete(e.App.Data, "correlationID")

	if l.isTerminal {
		fmt.Fprint(l.out, e.TerminalOutput())
	} else {
		_ = json.NewEncoder(l.out).Encode(e)
	}
}

func isJSON(s interface{}) (ok bool, hashmap map[string]interface{}) {
	var js map[string]interface{}

	sBytes, _ := json.Marshal(s)

	return json.Unmarshal(sBytes, &js) == nil, js
}

// Log logs messages at the default level.
func (l *logger) Log(args ...interface{}) {
	l.log(Info, "", args...)
}

// Logf logs formatted messages at default level.
func (l *logger) Logf(format string, args ...interface{}) {
	l.log(Info, format, args...)
}

// Info logs messages at the INFO level.
func (l *logger) Info(args ...interface{}) {
	l.log(Info, "", args...)
}

// Infof logs formatted messages at the INFO level.
func (l *logger) Infof(format string, args ...interface{}) {
	l.log(Info, format, args...)
}

// Debug logs messages at the DEBUG level.
func (l *logger) Debug(args ...interface{}) {
	l.log(Debug, "", args...)
}

// Debugf logs formatted messages at the DEBUG level.
func (l *logger) Debugf(format string, args ...interface{}) {
	l.log(Debug, format, args...)
}

// Warn logs messages at the WARN level.
func (l *logger) Warn(args ...interface{}) {
	l.log(Warn, "", args...)
}

// Warnf logs formatted messages at the WARN level.
func (l *logger) Warnf(format string, args ...interface{}) {
	l.log(Warn, format, args...)
}

// Error logs messages at the ERROR level and captures the stack trace.
func (l *logger) Error(args ...interface{}) {
	l.AddData("StackTrace", string(debug.Stack()))
	l.log(Error, "", args...)
	l.removeData("StackTrace")
}

// Errorf logs formatted messages at the ERROR level and captures the stack trace.
func (l *logger) Errorf(format string, args ...interface{}) {
	l.AddData("StackTrace", string(debug.Stack()))
	l.log(Error, format, args...)
	l.removeData("StackTrace")
}

// Fatal logs messages at the FATAL level, captures the stack trace, and exits the application.
func (l *logger) Fatal(args ...interface{}) {
	l.AddData("StackTrace", string(debug.Stack()))
	l.log(Fatal, "", args...)
	os.Exit(1)
}

// Fatalf logs formatted messages at the FATAL level, captures the stack trace, and exits the application.
func (l *logger) Fatalf(format string, args ...interface{}) {
	l.AddData("StackTrace", string(debug.Stack()))
	l.log(Fatal, format, args...)
	os.Exit(1)
}

// AddData adds key-value data to the logger's application data.
func (l *logger) AddData(key string, value interface{}) {
	l.app.syncData.Store(key, value)
}

// removeData removes the specified key from the logger's application data.
func (l *logger) removeData(key string) {
	l.app.syncData.Delete(key)
}
