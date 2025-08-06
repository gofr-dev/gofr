package dbresolver

import (
	"errors"
	"math/rand"
	"sync/atomic"

	"gofr.dev/pkg/gofr/container"
)

var errNoReplicasAvailable = errors.New("no replicas available")

// Strategy interface defines replica selection logic.
type Strategy interface {
	Choose(replicas []container.DB) (container.DB, error)
	Name() string
}

// RoundRobinStrategy selects replicas in round-robin order.
type RoundRobinStrategy struct {
	current atomic.Uint64
	count   int
}

// NewRoundRobinStrategy creates a new round-robin strategy instance.
func NewRoundRobinStrategy(count int) Strategy {
	return &RoundRobinStrategy{
		count: count,
	}
}

// Choose selects the next replica in round-robin fashion.
func (s *RoundRobinStrategy) Choose(replicas []container.DB) (container.DB, error) {
	if len(replicas) == 0 {
		return nil, errNoReplicasAvailable
	}

	current := s.current.Add(1)
	idx := int(current % uint64(len(replicas)))

	if len(replicas) > 0 {
		idx %= len(replicas)
	}

	return replicas[idx], nil
}

// Name returns the name of strategy.
func (*RoundRobinStrategy) Name() string {
	return roundRobinStrategy
}

// RandomStrategy selects a replica randomly.
type RandomStrategy struct{}

// NewRandomStrategy creates a new random strategy instance.
func NewRandomStrategy() Strategy {
	return &RandomStrategy{}
}

// Choose selects a random replica.
func (*RandomStrategy) Choose(replicas []container.DB) (container.DB, error) {
	if len(replicas) == 0 {
		return nil, errNoReplicasAvailable
	}

	return replicas[rand.Intn(len(replicas))], nil //nolint:gosec // acceptable randomness for load balance
}

// Name returns the name of the strategy.
func (*RandomStrategy) Name() string {
	return randomStrategy
}
