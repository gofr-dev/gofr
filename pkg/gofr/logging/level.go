// Package logging provides logging functionalities for GoFr applications.
package logging

import (
	"bytes"
	"strings"
)

// Level represents different logging levels.
type Level int

const (
	DEBUG Level = iota + 1
	INFO
	NOTICE
	WARN
	ERROR
	FATAL
)

// String constants for logging levels.
const (
	levelDEBUG  = "DEBUG"
	levelINFO   = "INFO"
	levelNOTICE = "NOTICE"
	levelWARN   = "WARN"
	levelERROR  = "ERROR"
	levelFATAL  = "FATAL"
)

// String returns the string representation of the log level.
func (l Level) String() string {
	switch l {
	case DEBUG:
		return levelDEBUG
	case INFO:
		return levelINFO
	case NOTICE:
		return levelNOTICE
	case WARN:
		return levelWARN
	case ERROR:
		return levelERROR
	case FATAL:
		return levelFATAL
	default:
		return ""
	}
}

//nolint:mnd // Color codes are sent as numbers
func (l Level) color() uint {
	switch l {
	case ERROR, FATAL:
		return 160
	case WARN, NOTICE:
		return 220
	case INFO:
		return 6
	case DEBUG:
		return 8
	default:
		return 37
	}
}

func (l Level) MarshalJSON() ([]byte, error) {
	buffer := bytes.NewBufferString(`"`)
	buffer.WriteString(l.String())
	buffer.WriteString(`"`)

	return buffer.Bytes(), nil
}

// GetLevelFromString converts a string to a logging level.
func GetLevelFromString(level string) Level {
	switch strings.ToUpper(level) {
	case levelDEBUG:
		return DEBUG
	case levelINFO:
		return INFO
	case levelNOTICE:
		return NOTICE
	case levelWARN:
		return WARN
	case levelERROR:
		return ERROR
	case levelFATAL:
		return FATAL
	default:
		return INFO
	}
}
