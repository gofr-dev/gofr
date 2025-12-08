package logging

import (
	"runtime/debug"
)

type PanicLog struct {
	Error      string `json:"error,omitempty"`
	StackTrace string `json:"stack_trace,omitempty"`
}

// LogPanic logs the panic error and stack trace using the provided logger.
func LogPanic(re any, logger Logger) {
	if re == nil {
		return
	}

	var e string
	switch t := re.(type) {
	case string:
		e = t
	case error:
		e = t.Error()
	default:
		e = "Unknown panic type"
	}

	logger.Error(PanicLog{
		Error:      e,
		StackTrace: string(debug.Stack()),
	})
}
