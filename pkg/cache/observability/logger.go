package observability

import "log"

// Logger is the minimal logging interface the cache needs.
type Logger interface {
	// Errorf logs messages at the ERROR level.
	Errorf(format string, args ...interface{})
	// Warnf logs messages at the WARN level.
	Warnf(format string, args ...interface{})
	// Infof logs messages at the INFO level.
	Infof(format string, args ...interface{})
	// Debugf logs messages at the DEBUG level.
	Debugf(format string, args ...interface{})
}

// a Logger that discards all messages.
type nopLogger struct{}

// returns a Logger that does nothing.
func NewNopLogger() Logger {
	return &nopLogger{}
}

func (l *nopLogger) Errorf(_ string, _ ...interface{}) {}
func (l *nopLogger) Warnf(_ string, _ ...interface{})  {}
func (l *nopLogger) Infof(_ string, _ ...interface{})  {}
func (l *nopLogger) Debugf(_ string, _ ...interface{}) {}

// wraps the standard library logger to satisfy Logger.
type stdLogger struct{}

// returns a Logger that writes to the standard log package.
func NewStdLogger() Logger {
	return &stdLogger{}
}

func (l *stdLogger) Errorf(format string, args ...interface{}) {
	log.Printf("[ERROR] "+format, args...)
}
func (l *stdLogger) Warnf(format string, args ...interface{}) {
	log.Printf("[WARN] "+format, args...)
}
func (l *stdLogger) Infof(format string, args ...interface{}) {
	log.Printf("[INFO] "+format, args...)
}
func (l *stdLogger) Debugf(format string, args ...interface{}) {
	log.Printf("[DEBUG] "+format, args...)
}
