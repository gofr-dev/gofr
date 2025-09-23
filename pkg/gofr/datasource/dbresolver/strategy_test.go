package dbresolver

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
	"gofr.dev/pkg/gofr/container"
)

func TestRoundRobinStrategy_Choose(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	// Create mock replicas
	mockReplica1 := NewMockDB(ctrl)
	mockReplica2 := NewMockDB(ctrl)
	mockReplica3 := NewMockDB(ctrl)

	replicas := []container.DB{mockReplica1, mockReplica2, mockReplica3}

	// Create strategy
	strategy := NewRoundRobinStrategy()

	// First call should return first replica
	db, err := strategy.Choose(replicas)
	require.NoError(t, err)
	assert.Equal(t, mockReplica1, db)

	// Second call should return second replica
	db, err = strategy.Choose(replicas)
	require.NoError(t, err)
	assert.Equal(t, mockReplica2, db)

	// Third call should return third replica
	db, err = strategy.Choose(replicas)
	require.NoError(t, err)
	assert.Equal(t, mockReplica3, db)

	// Fourth call should wrap around to first replica
	db, err = strategy.Choose(replicas)
	require.NoError(t, err)
	assert.Equal(t, mockReplica1, db)
}

func TestRoundRobinStrategy_Choose_SingleReplica(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	// Create mock replica
	mockReplica := NewMockDB(ctrl)
	replicas := []container.DB{mockReplica}

	// Create strategy
	strategy := NewRoundRobinStrategy()

	// All calls should return the same replica
	for i := 0; i < 5; i++ {
		db, err := strategy.Choose(replicas)
		require.NoError(t, err)
		assert.Equal(t, mockReplica, db)
	}
}

func TestRoundRobinStrategy_Choose_NoReplicas(t *testing.T) {
	strategy := NewRoundRobinStrategy()

	db, err := strategy.Choose([]container.DB{})
	assert.Nil(t, db)
	assert.ErrorIs(t, err, errNoReplicasAvailable)
}

func TestRandomStrategy_Choose(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	// Create mock replicas
	mockReplica1 := NewMockDB(ctrl)
	mockReplica2 := NewMockDB(ctrl)
	mockReplica3 := NewMockDB(ctrl)

	replicas := []container.DB{mockReplica1, mockReplica2, mockReplica3}

	// Create strategy
	strategy := NewRandomStrategy()

	// Verify that multiple calls return one of the replicas
	for i := 0; i < 10; i++ {
		result, err := strategy.Choose(replicas)
		require.NoError(t, err)
		assert.Contains(t, replicas, result)
	}
}

func TestRandomStrategy_Choose_SingleReplica(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	// Create mock replica
	mockReplica := NewMockDB(ctrl)
	replicas := []container.DB{mockReplica}

	// Create strategy
	strategy := NewRandomStrategy()

	// All calls should return the same replica
	for i := 0; i < 5; i++ {
		db, err := strategy.Choose(replicas)
		require.NoError(t, err)
		assert.Equal(t, mockReplica, db)
	}
}

func TestRandomStrategy_Choose_NoReplicas(t *testing.T) {
	strategy := NewRandomStrategy()

	db, err := strategy.Choose([]container.DB{})
	assert.Nil(t, db)
	assert.ErrorIs(t, err, errNoReplicasAvailable)
}

func TestStrategy_Name(t *testing.T) {
	roundRobin := NewRoundRobinStrategy()
	assert.Equal(t, string(StrategyRoundRobin), roundRobin.Name())

	random := NewRandomStrategy()
	assert.Equal(t, string(StrategyRandom), random.Name())
}
