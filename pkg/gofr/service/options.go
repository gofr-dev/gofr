package service

type Options interface {
	AddOption(h HTTP) HTTP
}

// Observable is optionally implemented by Options that need logger and metrics.
// NewHTTPService injects these automatically before calling AddOption.
type Observable interface {
	UseLogger(logger Logger)
	UseMetrics(metrics Metrics)
}
