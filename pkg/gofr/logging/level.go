package logging

import (
	"bytes"
	"strings"
)

type Level int

const (
	DEBUG Level = iota + 1
	INFO
	NOTICE
	WARN
	ERROR
	FATAL

	// String constants for logging levels.
	levelDEBUG  = "DEBUG"
	levelINFO   = "INFO"
	levelNOTICE = "NOTICE"
	levelWARN   = "WARN"
	levelERROR  = "ERROR"
	levelFATAL  = "FATAL"
)

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

//nolint:gomnd // Color codes are sent as numbers
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

func GetLevelFromString(level string) Level {
	switch strings.ToUpper(level) {
	case "DEBUG":
		return DEBUG
	case "INFO":
		return INFO
	case "NOTICE":
		return NOTICE
	case "WARN":
		return WARN
	case "ERROR":
		return ERROR
	case "FATAL":
		return FATAL
	default:
		return INFO
	}
}
