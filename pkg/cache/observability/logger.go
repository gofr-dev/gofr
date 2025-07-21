// package observability provides support for logging and metrics for the cache components.
package observability

import "log"

// Logger defines a standard interface for logging messages from cache components.
// It allows users to integrate their own logging solutions with the cache.
type Logger interface {
	// Errorf logs a formatted error message.
	Errorf(format string, args ...interface{})
	// Warnf logs a formatted warning message.
	Warnf(format string, args ...interface{})
	// Infof logs a formatted informational message.
	Infof(format string, args ...interface{})
	// Debugf logs a formatted debug message.
	Debugf(format string, args ...interface{})
}

// nopLogger is an implementation of Logger that discards all log messages.
// It is useful for disabling logging entirely.
type nopLogger struct{}

// NewNopLogger returns a logger that performs no operations.
func NewNopLogger() Logger {
	return &nopLogger{}
}

// Errorf does nothing.
func (l *nopLogger) Errorf(_ string, _ ...interface{}) {}

// Warnf does nothing.
func (l *nopLogger) Warnf(_ string, _ ...interface{}) {}

// Infof does nothing.
func (l *nopLogger) Infof(_ string, _ ...interface{}) {}

// Debugf does nothing.
func (l *nopLogger) Debugf(_ string, _ ...interface{}) {}

// stdLogger is an implementation of Logger that wraps the standard `log` package.
type stdLogger struct{}

// NewStdLogger returns a logger that writes messages to the standard logger.
// Log messages are prefixed with their severity level (e.g., [ERROR]).
func NewStdLogger() Logger {
	return &stdLogger{}
}

// Errorf logs a message at the ERROR level using the standard logger.
func (l *stdLogger) Errorf(format string, args ...interface{}) {
	log.Printf("[ERROR] "+format, args...)
}

// Warnf logs a message at the WARN level using the standard logger.
func (l *stdLogger) Warnf(format string, args ...interface{}) {
	log.Printf("[WARN] "+format, args...)
}

// Infof logs a message at the INFO level using the standard logger.
func (l *stdLogger) Infof(format string, args ...interface{}) {
	log.Printf("[INFO] "+format, args...)
}

// Debugf logs a message at the DEBUG level using the standard logger.
func (l *stdLogger) Debugf(format string, args ...interface{}) {
	log.Printf("[DEBUG] "+format, args...)
}
