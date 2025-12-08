package rbac

// Logger defines the logging interface for RBAC.
// It matches GoFr's logging.Logger interface methods used by RBAC.
type Logger interface {
	Debug(args ...any)
	Debugf(format string, args ...any)
	Info(args ...any)
	Infof(format string, args ...any)
	Error(args ...any)
	Errorf(format string, args ...any)
	Warn(args ...any)
	Warnf(format string, args ...any)
}
