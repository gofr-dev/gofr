package rbac

// Logger interface is used by RBAC package to log information.
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
