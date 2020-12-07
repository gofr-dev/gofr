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
)

func (l level) String() string {
	switch l {
	case DEBUG:
		return "DEBUG"
	case INFO:
		return "INFO"
	case NOTICE:
		return "NOTICE"
	case WARN:
		return "WARN"
	case ERROR:
		return "ERROR"
	case FATAL:
		return "FATAL"
	default:
		return "INFO"
	}
}

func (l level) color() uint {
	switch l {
	case ERROR, FATAL:
		return 31
	case WARN, NOTICE:
		return 33
	case INFO:
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
