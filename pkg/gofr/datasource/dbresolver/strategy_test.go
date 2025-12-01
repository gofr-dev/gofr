package dbresolver

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestRoundRobinStrategy_Next(t *testing.T) {
	strategy := NewRoundRobinStrategy()

	// Test with 3 replicas
	count := 3

	// First call should return index 0
	idx := strategy.Next(count)

	assert.Equal(t, 0, idx)

	// Second call should return index 1
	idx = strategy.Next(count)

	assert.Equal(t, 1, idx)

	// Third call should return index 2
	idx = strategy.Next(count)

	assert.Equal(t, 2, idx)

	// Fourth call should wrap around to index 0
	idx = strategy.Next(count)

	assert.Equal(t, 0, idx)
}

func TestRoundRobinStrategy_Next_SingleReplica(t *testing.T) {
	strategy := NewRoundRobinStrategy()

	// All calls should return index 0
	for i := 0; i < 5; i++ {
		idx := strategy.Next(1)

		assert.Equal(t, 0, idx)
	}
}

func TestRoundRobinStrategy_Next_NoReplicas(t *testing.T) {
	strategy := NewRoundRobinStrategy()

	idx := strategy.Next(0)

	assert.Equal(t, -1, idx)
}

func TestRandomStrategy_Next(t *testing.T) {
	strategy := NewRandomStrategy()

	count := 3

	// Verify that multiple calls return valid indices
	seen := make(map[int]bool)

	for i := 0; i < 100; i++ {
		idx := strategy.Next(count)

		assert.GreaterOrEqual(t, idx, 0)
		assert.Less(t, idx, count)

		seen[idx] = true
	}

	// With 100 iterations, we should see all 3 indices
	assert.Len(t, seen, count)
}

func TestRandomStrategy_Next_SingleReplica(t *testing.T) {
	strategy := NewRandomStrategy()

	// All calls should return index 0
	for i := 0; i < 5; i++ {
		idx := strategy.Next(1)

		assert.Equal(t, 0, idx)
	}
}

func TestRandomStrategy_Next_NoReplicas(t *testing.T) {
	strategy := NewRandomStrategy()

	idx := strategy.Next(0)

	assert.Equal(t, -1, idx)
}

func TestStrategy_Name(t *testing.T) {
	roundRobin := NewRoundRobinStrategy()

	assert.Equal(t, string(StrategyRoundRobin), roundRobin.Name())

	random := NewRandomStrategy()

	assert.Equal(t, string(StrategyRandom), random.Name())
}
