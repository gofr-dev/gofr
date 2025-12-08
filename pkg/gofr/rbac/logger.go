package rbac

// Logger defines methods for logging that RBAC uses.
// This interface matches the methods used by RBAC for audit logging and error reporting.
type Logger interface {
	// Infof logs a formatted info message.
	Infof(pattern string, args ...any)
	// Errorf logs a formatted error message.
	Errorf(pattern string, args ...any)
	// Warnf logs a formatted warning message.
	Warnf(pattern string, args ...any)
	// Debugf logs a formatted debug message.
	Debugf(pattern string, args ...any)
}

