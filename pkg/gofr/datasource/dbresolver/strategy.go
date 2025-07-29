package dbresolver

import (
	"math/rand"
	"sync/atomic"

	"gofr.dev/pkg/gofr/container"
)

// Strategy defines how to choose a replica from available replicas.
type Strategy interface {
	Choose(replicas []container.DB) container.DB
	Name() string
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
func (s *RoundRobinStrategy) Choose(replicas []container.DB) container.DB {
	if len(replicas) == 0 {
		panic("no replicas available")
	}

	idx := int(s.current.Add(1)) % len(replicas)

	return replicas[idx]
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
func (*RandomStrategy) Choose(replicas []container.DB) container.DB {
	if len(replicas) == 0 {
		panic("no replicas available")
	}

	return replicas[rand.Intn(len(replicas))] //nolint:gosec // weak RNG is okay for load balancing
}

// Name returns the strategy name.
func (*RandomStrategy) Name() string {
	return randomStrategy
}
