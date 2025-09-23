package dbresolver

import (
	"errors"
	"math/rand"
	"sync/atomic"

	"gofr.dev/pkg/gofr/container"
)

// StrategyType defines the load balancing strategy type.
type StrategyType string

const (
	// StrategyRoundRobin distributes load sequentially across replicas.
	StrategyRoundRobin StrategyType = "round-robin"
	// StrategyRandom randomly selects a replica for each request.
	StrategyRandom StrategyType = "random"
)

var errNoReplicasAvailable = errors.New("no replicas available")

// Strategy interface defines replica selection logic.
type Strategy interface {
	Choose(replicas []container.DB) (container.DB, error)
	Name() string
}

// RoundRobinStrategy selects replicas in round-robin order.
type RoundRobinStrategy struct {
	current atomic.Int64
}

// NewRoundRobinStrategy creates a new round-robin strategy instance.
func NewRoundRobinStrategy() Strategy {
	return &RoundRobinStrategy{}
}

// Choose selects the next replica in round-robin fashion.
// Choose selects the next replica in round-robin fashion.
func (s *RoundRobinStrategy) Choose(replicas []container.DB) (container.DB, error) {
	replicaCount := int64(len(replicas))
	if replicaCount == 0 {
		return nil, errNoReplicasAvailable
	}

	count := s.current.Add(1)

	idx64 := count % replicaCount

	idx := int(idx64)

	return replicas[idx], nil
}

// Name returns the name of strategy.
func (*RoundRobinStrategy) Name() string {
	return string(StrategyRoundRobin)
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
	return string(StrategyRandom)
}
