package dbresolver

import (
	"gofr.dev/pkg/gofr/container"
)

// ResolverWrapper implements container.Provider interface
// and acts as an adapter to the actual Resolver implementation
type ResolverWrapper struct {
	logger       Logger
	metrics      Metrics
	tracer       interface{}
	strategy     string
	readFallback bool
}

// NewProvider creates a new resolver provider
func NewProvider(strategy string, readFallback bool) *ResolverWrapper {
	return &ResolverWrapper{
		strategy:     strategy,
		readFallback: readFallback,
	}
}

// UseLogger sets the logger for the resolver
func (r *ResolverWrapper) UseLogger(logger interface{}) {
	if l, ok := logger.(Logger); ok {
		r.logger = l
	}
}

// UseMetrics sets the metrics for the resolver
func (r *ResolverWrapper) UseMetrics(metrics interface{}) {
	if m, ok := metrics.(Metrics); ok {
		r.metrics = m
	}
}

// UseTracer sets the tracer for the resolver
func (r *ResolverWrapper) UseTracer(tracer interface{}) {
	r.tracer = tracer
}

// Connect is a no-op as we don't need to establish connections
func (r *ResolverWrapper) Connect() {
	// No-op - we don't create connections, we use existing ones
}

// Build creates a resolver with the given primary and replicas
func (r *ResolverWrapper) Build(primary container.DB, replicas []container.DB) container.DB {
	if primary == nil {
		panic("primary database cannot be nil")
	}

	// Create options for the resolver
	opts := []Option{
		WithStrategy(r.strategy),
		WithReadFallback(r.readFallback),
	}

	// Add strategy option
	opts = append(opts, WithStrategy(r.strategy))

	// Add read fallback option
	opts = append(opts, WithReadFallback(r.readFallback))

	// Create and return the resolver
	return New(primary, replicas, r.logger, r.metrics, opts...)
}
