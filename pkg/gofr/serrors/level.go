package serrors

type Level int

const (
	INFO Level = iota + 1
	WARNING
	ERROR
	CRITICAL
)

func (level Level) GetErrorLevel() string {
	switch level {
	case INFO:
		return "INFO"
	case WARNING:
		return "WARNING" //nolint:goconst // used only for testing
	case ERROR:
		return "ERROR"
	case CRITICAL:
		return "CRITICAL"
	default:
		return "UNKNOWN"
	}
}
