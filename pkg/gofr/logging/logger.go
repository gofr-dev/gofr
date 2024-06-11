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

const (
	fileMode       = 0644
	passwordLength = 10
)

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
	ChangeLevel(level Level)
	SetMaskingFilters(fields []string)
	GetMaskingFilters() []string
}

// Filterer represents an interface to filter log messages.
type Filterer interface {
	Filter(message interface{}) interface{}
}

// MaskingFilter is an implementation of the Filterer interface that masks sensitive fields.
type MaskingFilter struct {
	// MaskFields is a slice of fields to mask, e.g. ["password", "credit_card_number"]
	MaskFields []string
}

func (f *MaskingFilter) Filter(message interface{}) interface{} {
	// Get the value of the message using reflection
	val := reflect.ValueOf(message)

	// If the message is a pointer, get the underlying value
	if val.Kind() == reflect.Ptr {
		val = val.Elem()
	}

	// Handle slices and structs differently
	//nolint:exhaustive // This switch statement is intentionally not exhaustive to handle only the relevant cases
	switch val.Kind() {
	case reflect.Slice:
		// Create a new slice to store the filtered elements
		newSlice := reflect.MakeSlice(val.Type(), val.Len(), val.Len())

		// Iterate over the slice elements and filter each element
		for i := 0; i < val.Len(); i++ {
			elem := val.Index(i)
			filteredElem := f.filterElement(elem)
			newSlice.Index(i).Set(reflect.ValueOf(filteredElem))
		}

		return newSlice.Interface()

	case reflect.Struct:
		// Create a new copy of the struct value
		newVal := reflect.New(val.Type()).Elem()
		newVal.Set(val)

		// Recursively filter the struct fields
		f.filterFields(newVal)

		// If the original message was a pointer, return a pointer to the new value
		if message != nil && reflect.TypeOf(message).Kind() == reflect.Ptr {
			return newVal.Addr().Interface()
		}

		return newVal.Interface()

	default:
		// For other types, return the original message
		return message
	}
}

func (f *MaskingFilter) filterElement(val reflect.Value) interface{} {
	// If the element is a pointer, get the underlying value
	if val.Kind() == reflect.Ptr {
		val = val.Elem()
	}

	// Handle interfaces, structs, and other types differently
	//nolint:exhaustive // This switch statement is intentionally not exhaustive to handle only the relevant cases
	switch val.Kind() {
	case reflect.Interface:
		// Get the underlying value of the interface
		underlyingVal := val.Elem()
		// Recursively filter the underlying value
		filteredVal := f.filterElement(underlyingVal)

		return filteredVal

	case reflect.Struct:
		// Create a new copy of the struct value
		newVal := reflect.New(val.Type()).Elem()
		newVal.Set(val)

		// Recursively filter the struct fields
		f.filterFields(newVal)

		return newVal.Interface()

	default:
		// For other types, return the original element
		return val.Interface()
	}
}

func (f *MaskingFilter) filterFields(val reflect.Value) {
	for i := 0; i < val.NumField(); i++ {
		field := val.Field(i)
		fieldType := val.Type().Field(i)

		// If the field is a pointer, get the underlying value
		if field.Kind() == reflect.Ptr {
			field = field.Elem()
		}

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

func (f *MaskingFilter) maskField(field reflect.Value, fieldName string) {
	//nolint:exhaustive // Only handling specific types needed for masking
	switch field.Kind() {
	case reflect.String:
		if fieldName == "Password" {
			field.SetString(maskString(field.String(), passwordLength))
		} else {
			field.SetString(maskString(field.String()))
		}
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		field.SetInt(0)
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		field.SetUint(0)
	case reflect.Float32, reflect.Float64:
		field.SetFloat(0)
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
	level         Level
	normalOut     io.Writer
	errorOut      io.Writer
	isTerminal    bool
	lock          chan struct{}
	filter        Filterer
	maskingFields []string
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

	out := l.getOutputWriter(level)
	entry := l.createLogEntry(level, format, args...)

	if l.filter != nil {
		entry.Message = l.filter.Filter(entry.Message)
	}

	l.writeLogEntry(entry, out)
}

func (l *logger) getOutputWriter(level Level) io.Writer {
	if level >= ERROR {
		return l.errorOut
	}

	return l.normalOut
}

func (l *logger) createLogEntry(level Level, format string, args ...interface{}) logEntry {
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

	return entry
}

func (l *logger) writeLogEntry(entry logEntry, out io.Writer) {
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
	// Note: we need to lock the pretty print as printing to stdandard output not concurency safe
	// the logs when printed in go routines were getting missaligned since we are achieveing
	// a single line of log, in 2 separate statements which caused the missalignment.
	l.lock <- struct{}{} // Acquire the channel's lock
	defer func() {
		<-l.lock // Release the channel's token
	}()

	// Pretty printing if the message interface defines a method PrettyPrint else print the log message
	// This decouples the logger implementation from its usage
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
func NewLogger(level Level) Logger {
	l := &logger{
		normalOut: os.Stdout,
		errorOut:  os.Stderr,
		lock:      make(chan struct{}, 1),
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

func (l *logger) ChangeLevel(level Level) {
	l.level = level
}

// SetMaskingFilters sets the masking fields and enables masking for the logger.
func (l *logger) SetMaskingFilters(fields []string) {
	l.maskingFields = fields
	l.filter = &MaskingFilter{
		MaskFields: fields,
	}
}

// GetMaskingFilters returns the current masking fields.
func (l *logger) GetMaskingFilters() []string {
	return l.maskingFields
}
