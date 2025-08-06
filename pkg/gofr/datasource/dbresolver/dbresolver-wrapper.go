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
	strategyName string
	readFallback bool
}

// NewProvider creates a new ResolverWrapper with strategy and fallback config.
func NewProvider(strategy string, readFallback bool) *ResolverWrapper {
	return &ResolverWrapper{
		strategyName: strategy,
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

// createStrategy creates a Strategy instance from string name.
func (r *ResolverWrapper) createStrategy(replicaCount int) Strategy {
	switch r.strategyName {
	case roundRobinStrategy:
		return NewRoundRobinStrategy(replicaCount)
	case randomStrategy:
		return NewRandomStrategy()
	default:
		// Default to round-robin if unknown strategy.
		return NewRoundRobinStrategy(replicaCount)
	}
}

// Build creates a Resolver instance with primary and replica DBs.
func (r *ResolverWrapper) Build(primary container.DB, replicas []container.DB) (container.DB, error) {
	if primary == nil {
		return nil, errPrimaryNil
	}

	// Create options slice
	var opts []Option

	var strategy Strategy
	if r.strategyName == "random" {
		strategy = NewRandomStrategy()
	} else {
		// Default to round-robin for any other value including "round-robin"
		strategy = NewRoundRobinStrategy(len(replicas))
	}

	// Add options.
	opts = append(opts, WithStrategy(strategy), WithFallback(r.readFallback))

	// Create and return the resolver.
	return NewResolver(primary, replicas, r.logger, r.metrics, opts...), nil
}
