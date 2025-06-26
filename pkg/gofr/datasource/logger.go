package datasource

// Logger interface is used by datasource packages to log information about query execution.
// Developer Notes: Note that it's a reduced version of logging.Logger interface. We are not using that package to
// ensure that datasource package is not dependent on logging package. That way logging package should be easily able
// to import datasource package and provide a different "pretty" version for different log types defined here while
// avoiding the cyclical import issue. Idiomatically, interfaces should be defined by packages who are using it; unlike
// other languages. Also - accept interfaces, return concrete types.
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
