package dbresolver

import (
	"errors"

	"go.opentelemetry.io/otel/trace"
	"gofr.dev/pkg/gofr/container"
)

var (
	errPrimaryNil              = errors.New("primary database cannot be nil")
	errReplicaFailedNoFallback = errors.New("replica query failed and fallback disabled")
)

// ResolverWrapper implements container.DBResolverProvider interface
// It acts as a factory that creates a Resolver with given config.
type ResolverWrapper struct {
	logger       Logger
	metrics      Metrics
	tracer       trace.Tracer
	strategy     Strategy
	readFallback bool
}

// NewProvider creates a new ResolverWrapper with strategy and fallback config.
func NewProvider(strategy Strategy, readFallback bool) *ResolverWrapper {
	return &ResolverWrapper{
		strategy:     strategy,
		readFallback: readFallback,
	}
}

// UseLogger sets the logger instance.
func (r *ResolverWrapper) UseLogger(logger any) {
	if l, ok := logger.(Logger); ok {
		r.logger = l
	}
}

// UseMetrics sets the metrics instance.
func (r *ResolverWrapper) UseMetrics(metrics any) {
	if m, ok := metrics.(Metrics); ok {
		r.metrics = m
	}
}

// UseTracer sets the tracer instance.
func (r *ResolverWrapper) UseTracer(tracer any) {
	if t, ok := tracer.(trace.Tracer); ok {
		r.tracer = t
	}
}

// Connect is no-op for wrapper as connections are created externally.
func (*ResolverWrapper) Connect() {
	// no-op
}

// Build creates a Resolver instance with primary and replica DBs.
func (r *ResolverWrapper) Build(primary container.DB, replicas []container.DB) (container.DB, error) {
	if primary == nil {
		return nil, errPrimaryNil
	}

	// Create options slice
	var opts []Option

	// Add options.
	opts = append(opts, WithStrategy(r.strategy), WithFallback(r.readFallback))

	// Create and return the resolver.
	return NewResolver(primary, replicas, r.logger, r.metrics, opts...), nil
}
