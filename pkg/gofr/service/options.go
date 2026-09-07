package service

type Options interface {
	AddOption(h HTTP) HTTP
}

// Observable is an optional interface that Options may implement to receive
// the HTTP service's Logger and Metrics. NewHTTPService calls SetLogger and
// SetMetrics on any option that implements Observable before invoking
// AddOption, so options that need out-of-band observability (typically those
// that spin up background goroutines) do not have to plumb a logger or metrics
// instance through their constructor.
//
// When an Options value is applied directly via AddOption — outside
// NewHTTPService — callers must set the logger and metrics themselves.
type Observable interface {
	SetLogger(Logger)
	SetMetrics(Metrics)
}
