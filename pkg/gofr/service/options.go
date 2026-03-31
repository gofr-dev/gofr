package service

type Options interface {
	// AddOption wraps the given HTTP service with this option's behavior.
	//
	// When options are passed to NewHTTPService (via AddHTTPService), logger and
	// metrics are injected automatically into options that implement Observable.
	// When calling AddOption directly, callers must invoke UseLogger and UseMetrics
	// manually if the option supports them.
	AddOption(h HTTP) HTTP
}

// Observable is optionally implemented by Options that need logger and metrics.
// NewHTTPService injects these automatically before calling AddOption.
type Observable interface {
	UseLogger(logger Logger)
	UseMetrics(metrics Metrics)
}
