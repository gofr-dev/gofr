package dbresolver

import (
	"errors"
	"math/rand"
	"sync/atomic"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
	"gofr.dev/pkg/gofr/container"
)

var errNoReplicasAvailable = errors.New("no replicas available")

// Strategy defines how to choose a replica from available replicas.
type Strategy interface {
	Choose(replicas []container.DB) (container.DB, error)
	Name() string
}

// chooseReplicaOrFallback attempts to select a replica and handles fallback logic.
// It returns either a selected replica DB or the primary DB if fallback occurs.
func (r *Resolver) chooseReplicaOrFallback(span trace.Span) container.DB {
	if len(r.replicas) == 0 {
		// No replicas available, use primary
		if span != nil {
			span.SetAttributes(attribute.String("dbresolver.target", "primary"))
		}
		r.stats.primaryReads.Add(1)
		return r.primary
	}

	db, err := r.strategy.Choose(r.replicas)
	if err != nil {
		// Replica selection failed, fall back to primary
		r.logger.Debugf("Failed to choose replica: %v, falling back to primary", err)
		r.stats.replicaFailures.Add(1)
		r.stats.primaryFallbacks.Add(1)
		r.stats.primaryReads.Add(1)

		if span != nil {
			span.SetAttributes(attribute.Bool("dbresolver.fallback", true))
			span.SetAttributes(attribute.String("dbresolver.target", "primary"))
		}

		return r.primary
	}

	if span != nil {
		span.SetAttributes(attribute.String("dbresolver.target", "replica"))
	}

	return db
}

// RoundRobinStrategy selects replicas in round-robin fashion.
type RoundRobinStrategy struct {
	current atomic.Int64
	count   int
}

// NewRoundRobinStrategy creates a new round-robin strategy.
func NewRoundRobinStrategy(count int) Strategy {
	return &RoundRobinStrategy{count: count}
}

// Choose selects a replica using round-robin.
func (s *RoundRobinStrategy) Choose(replicas []container.DB) (container.DB, error) {
	if len(replicas) == 0 {
		return nil, errNoReplicasAvailable
	}

	idx := int(s.current.Add(1)) % len(replicas)
	return replicas[idx], nil
}

// Name returns the strategy name.
func (*RoundRobinStrategy) Name() string {
	return roundRobinStrategy
}

// RandomStrategy selects replicas randomly.
type RandomStrategy struct{}

// NewRandomStrategy creates a new random strategy.
func NewRandomStrategy() Strategy {
	return &RandomStrategy{}
}

// Choose selects a replica randomly.
func (*RandomStrategy) Choose(replicas []container.DB) (container.DB, error) {
	if len(replicas) == 0 {
		return nil, errNoReplicasAvailable
	}

	return replicas[rand.Intn(len(replicas))], nil //nolint:gosec // weak RNG is okay for load balancing
}

// Name returns the strategy name.
func (*RandomStrategy) Name() string {
	return randomStrategy
}
