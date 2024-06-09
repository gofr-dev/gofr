package datasource

// Logger interface is used by datasource packages to log information about query execution.
// Developer Notes: Note that it's a reduced version of logging.Logger interface. We are not using that package to
// ensure that datasource package is not dependent on logging package. That way logging package should be easily able
// to import datasource package and provide a different "pretty" version for different log types defined here while
// avoiding the cyclical import issue. Idiomatically, interfaces should be defined by packages who are using it; unlike
// other languages. Also - accept interfaces, return concrete types.
type Logger interface {
	Debug(args ...interface{})
	Debugf(format string, args ...interface{})
	Log(args ...interface{})
	Logf(format string, args ...interface{})
	Error(args ...interface{})
	Errorf(format string, args ...interface{})
	Warn(args ...interface{})
	Warnf(format string, args ...interface{})
}
