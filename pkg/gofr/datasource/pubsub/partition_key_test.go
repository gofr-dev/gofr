package pubsub_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"gofr.dev/pkg/gofr/datasource/pubsub"
	"gofr.dev/pkg/gofr/datasource/pubsub/internal/partitionkey"
)

func TestWithPartitionKey(t *testing.T) {
	parent, cancel := context.WithCancel(t.Context())
	defer cancel()

	key := []byte("order-123")
	ctx := pubsub.WithPartitionKey(parent, key)
	key[0] = 'X'

	assert.Equal(t, []byte("order-123"), partitionkey.FromContext(ctx))
	assert.Nil(t, partitionkey.FromContext(parent))

	resolved := partitionkey.FromContext(ctx)
	resolved[0] = 'Y'

	assert.Equal(t, []byte("order-123"), partitionkey.FromContext(ctx))

	nested := pubsub.WithPartitionKey(ctx, []byte("order-456"))
	assert.Equal(t, []byte("order-456"), partitionkey.FromContext(nested))
	assert.Equal(t, []byte("order-123"), partitionkey.FromContext(ctx))
	cancel()
	require.ErrorIs(t, ctx.Err(), context.Canceled)
}

func TestEmptyPartitionKeyOverridesInheritedKey(t *testing.T) {
	parent := pubsub.WithPartitionKey(t.Context(), []byte("parent-key"))

	for _, key := range [][]byte{nil, {}} {
		assert.Nil(t, partitionkey.FromContext(pubsub.WithPartitionKey(parent, key)))
		assert.Equal(t, []byte("parent-key"), partitionkey.FromContext(parent))
	}
}
