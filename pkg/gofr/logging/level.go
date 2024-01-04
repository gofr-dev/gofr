package logging

import "bytes"

type level int

const (
	DEBUG level = iota + 1
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

func (l level) String() string {
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
func (l level) color() uint {
	switch l {
	case ERROR, FATAL:
		return 31
	case WARN, NOTICE:
		return 33
	case INFO:
		return 36
	case DEBUG:
		return 36
	default:
		return 37
	}
}

func (l level) MarshalJSON() ([]byte, error) {
	buffer := bytes.NewBufferString(`"`)
	buffer.WriteString(l.String())
	buffer.WriteString(`"`)

	return buffer.Bytes(), nil
}
