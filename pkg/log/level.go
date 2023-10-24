package log

import (
	"bytes"
	"strings"
)

type level int

const (
	Fatal level = iota + 1
	Error
	Warn
	Info
	Debug
)

const info = "INFO"

// String returns the string representation of a log level.
func (l level) String() string {
	logLevel := info

	switch l {
	case Fatal:
		logLevel = "FATAL"
	case Error:
		logLevel = "ERROR"
	case Warn:
		logLevel = "WARN"
	case Debug:
		logLevel = "DEBUG"
	case Info:
		logLevel = info
	}

	return logLevel
}

const (
	redColor    = 31
	yellowColor = 33
	blueColor   = 36
	normalColor = 37
)

// colorCode returns the color to be used for the formatting at terminal
func (l level) colorCode() int {
	colorCode := normalColor

	switch l {
	case Error, Fatal:
		colorCode = redColor
	case Warn:
		colorCode = yellowColor
	case Info:
		colorCode = blueColor
	case Debug:
		colorCode = normalColor
	}

	return colorCode
}

// MarshalJSON returns the JSON representation of a log level.
func (l level) MarshalJSON() ([]byte, error) {
	buffer := bytes.NewBufferString(`"`)
	buffer.WriteString(l.String())
	buffer.WriteString(`"`)

	return buffer.Bytes(), nil
}

// getLevel returns the log level based on the input string (case-insensitive).
func getLevel(level string) level {
	switch strings.ToUpper(level) {
	case info:
		return Info
	case "WARN":
		return Warn
	case "FATAL":
		return Fatal
	case "DEBUG":
		return Debug
	case "ERROR":
		return Error
	default:
		return Info
	}
}
