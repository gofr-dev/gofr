package dbresolver

import (
	"math/rand"
	"sync/atomic"
)

// StrategyType defines the load balancing strategy type.
type StrategyType string

const (
	// StrategyRoundRobin distributes load sequentially across replicas.
	StrategyRoundRobin StrategyType = "round-robin"
	// StrategyRandom randomly selects a replica for each request.
	StrategyRandom StrategyType = "random"
)

// Strategy interface defines replica selection logic.
type Strategy interface {
	Name() string
	Next(count int) int
}

// RoundRobinStrategy selects replicas in round-robin order.
type RoundRobinStrategy struct {
	current atomic.Uint64
}

// NewRoundRobinStrategy creates a new round-robin strategy instance.
func NewRoundRobinStrategy() Strategy {
	return &RoundRobinStrategy{}
}

// Next selects the next replica index in round-robin fashion.
func (s *RoundRobinStrategy) Next(count int) int {
	if count <= 0 {
		return -1
	}

	next := s.current.Add(1)

	return int((next - 1) % uint64(count)) //nolint:gosec // count is validated to be positive
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

// Next selects a random replica index.
func (*RandomStrategy) Next(count int) int {
	if count == 0 {
		return -1
	}

	return rand.Intn(count) //nolint:gosec // acceptable randomness for load balance
}

// Name returns the name of the strategy.
func (*RandomStrategy) Name() string {
	return string(StrategyRandom)
}
