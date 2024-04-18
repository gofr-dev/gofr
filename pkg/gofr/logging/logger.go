package logging

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"reflect"
	"strings"
	"time"

	"golang.org/x/term"

	"gofr.dev/pkg/gofr/version"
)

const fileMode = 0644

type PrettyPrint interface {
	PrettyPrint(writer io.Writer)
}

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

// Filterer represents an interface to filter log messages.
type Filterer interface {
	Filter(message interface{}) interface{}
}

// DefaultFilter is the default implementation of the Filterer interface.
type DefaultFilter struct {
	// MaskFields is a slice of fields to mask, e.g. ["password", "credit_card_number"]
	MaskFields []string
	// EnableMasking is a flag to enable or disable masking
	EnableMasking bool
}

func (f *DefaultFilter) Filter(message interface{}) interface{} {
	// Get the value of the message using reflection
	val := reflect.ValueOf(message)

	// If masking is disabled or the message is not a struct, return the original message
	if !f.EnableMasking || val.Kind() != reflect.Struct {
		return message
	}

	// Create a new copy of the struct value
	newVal := reflect.New(val.Type()).Elem()
	newVal.Set(val)

	// Recursively filter the struct fields
	f.filterFields(newVal)

	return newVal.Interface()
}
func (f *DefaultFilter) filterFields(val reflect.Value) {
	for i := 0; i < val.NumField(); i++ {
		field := val.Field(i)
		fieldType := val.Type().Field(i)

		// Check if the field name matches any of the mask fields (case-insensitive)
		fieldName := fieldType.Name
		if contains(f.MaskFields, fieldName) {
			// Mask the field value
			f.maskField(field, fieldName)
		} else if field.Kind() == reflect.Struct {
			// If the field is a struct, recursively filter its fields
			f.filterFields(field)
		}
	}
}

func (f *DefaultFilter) maskField(field reflect.Value, fieldName string) {
	switch field.Kind() {
	case reflect.String:
		if fieldName == "Password" {
			field.SetString(maskString(field.String(), 10)) // Mask password with fixed length of 10
		} else {
			field.SetString(maskString(field.String()))
		}
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		field.SetInt(0)
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		field.SetUint(0)
	case reflect.Float32, reflect.Float64:
		field.SetFloat(0)
		// Add more cases for other types if needed
	}
}
func contains(maskFields []string, fieldName string) bool {
	for _, field := range maskFields {
		if strings.EqualFold(field, fieldName) {
			return true
		}
	}
	return false
}

func maskString(str string, maskLength ...int) string {
	length := len(str)
	if len(maskLength) > 0 {
		length = maskLength[0]
	}
	masked := strings.Repeat("*", length)
	return masked
}

type logger struct {
	level      Level
	normalOut  io.Writer
	errorOut   io.Writer
	isTerminal bool
	filter     Filterer
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
		entry.Message = fmt.Sprintf(format+"", args...)
	}

	if l.filter != nil {
		entry.Message = l.filter.Filter(entry.Message)
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
	if fn, ok := e.Message.(PrettyPrint); ok {
		fmt.Fprintf(out, "\u001B[38;5;%dm%s\u001B[0m [%s] ", e.Level.color(), e.Level.String()[0:4],
			e.Time.Format("15:04:05"))

		fn.PrettyPrint(out)
	} else {
		fmt.Fprintf(out, "\u001B[38;5;%dm%s\u001B[0m [%s] ", e.Level.color(), e.Level.String()[0:4],
			e.Time.Format("15:04:05"))

		fmt.Fprintf(out, "%v\n", e.Message)
	}
}

// NewLogger creates a new logger instance with the specified logging level.
func NewLogger(level Level, args ...interface{}) Logger {
	var filter Filterer
	if len(args) > 0 {
		f, ok := args[0].(Filterer)
		if !ok {
			// If the provided argument does not implement the Filterer interface, use the default filter
			filter = &DefaultFilter{
				MaskFields:    []string{},
				EnableMasking: true,
			}
		} else {
			filter = f
		}
	} else {
		filter = &DefaultFilter{
			MaskFields:    []string{},
			EnableMasking: true,
		}
	}

	l := &logger{
		normalOut: os.Stdout,
		errorOut:  os.Stderr,
		level:     level,
		filter:    filter,
	}

	l.isTerminal = checkIfTerminal(l.normalOut)

	return l
}

// NewFileLogger creates a new logger instance with logging to a file.
func NewFileLogger(path string, args ...interface{}) Logger {
	var filter Filterer
	if len(args) > 0 {
		f, ok := args[0].(Filterer)
		if !ok {
			// If the provided argument does not implement the Filterer interface, use the default filter
			filter = &DefaultFilter{
				MaskFields:    []string{},
				EnableMasking: true,
			}
		} else {
			filter = f
		}
	} else {
		filter = &DefaultFilter{
			MaskFields:    []string{},
			EnableMasking: true,
		}
	}

	l := &logger{
		normalOut: io.Discard,
		errorOut:  io.Discard,
		filter:    filter,
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
