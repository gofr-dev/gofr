package dbresolver

import (
	"gofr.dev/pkg/gofr/container"
)

// ResolverWrapper implements container.Provider interface
// and acts as an adapter to the actual Resolver implementation.
type ResolverWrapper struct {
	logger       Logger
	metrics      Metrics
	tracer       any
	strategy     string
	readFallback bool
}

// NewProvider creates a new resolver provider.
func NewProvider(strategy string, readFallback bool) *ResolverWrapper {
	return &ResolverWrapper{
		strategy:     strategy,
		readFallback: readFallback,
	}
}

// UseLogger sets the logger for the resolver.
func (r *ResolverWrapper) UseLogger(logger any) {
	if l, ok := logger.(Logger); ok {
		r.logger = l
	}
}

// UseMetrics sets the metrics for the resolver.
func (r *ResolverWrapper) UseMetrics(metrics any) {
	if m, ok := metrics.(Metrics); ok {
		r.metrics = m
	}
}

// UseTracer sets the tracer for the resolver.
func (r *ResolverWrapper) UseTracer(tracer any) {
	r.tracer = tracer
}

// Connect is a no-op as we don't need to establish connections.
func (*ResolverWrapper) Connect() {
	// No-op - we don't create connections, we use existing ones
}

// createStrategy creates a Strategy instance from string name.
func (r *ResolverWrapper) createStrategy(replicaCount int) Strategy {
	switch r.strategy {
	case "round-robin":
		return NewRoundRobinStrategy(replicaCount)
	case "random":
		return NewRandomStrategy()
	default:
		// Default to round-robin if unknown strategy.
		return NewRoundRobinStrategy(replicaCount)
	}
}

// Build creates a resolver with the given primary and replicas.
func (r *ResolverWrapper) Build(primary container.DB, replicas []container.DB) container.DB {
	if primary == nil {
		panic("primary database cannot be nil")
	}

	// Create strategy instance based on string name.
	strategy := r.createStrategy(len(replicas))

	// Create options for the resolver.
	opts := []Option{
		WithStrategy(strategy),
		WithFallback(r.readFallback),
	}

	// Create and return the resolver.
	return New(primary, replicas, r.logger, r.metrics, opts...)
}
