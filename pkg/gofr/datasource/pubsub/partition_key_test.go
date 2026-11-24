package pubsub_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"gofr.dev/pkg/gofr/datasource/pubsub"
	"gofr.dev/pkg/gofr/datasource/pubsub/internal/keyfields"
)

func TestWithPartitionKey(t *testing.T) {
	parent, cancel := context.WithCancel(t.Context())
	defer cancel()

	key := []byte("order-123")
	ctx := pubsub.WithPartitionKey(parent, key)
	key[0] = 'X'
	fields := keyfields.Fields{"orders": "orderId"}
	logger := &silentKeyLogger{}
	assert.Equal(t, []byte("order-123"), fields.Resolve(ctx, "orders", []byte("binary payload"), logger))
	assert.Nil(t, fields.Resolve(parent, "unmapped", nil, logger), "parent context must not be modified")

	resolved := fields.Resolve(ctx, "orders", nil, logger)
	resolved[0] = 'Y'

	assert.Equal(t, []byte("order-123"), fields.Resolve(ctx, "orders", nil, logger))

	nested := pubsub.WithPartitionKey(ctx, []byte("order-456"))
	assert.Equal(t, []byte("order-456"), fields.Resolve(nested, "orders", nil, logger))
	assert.Equal(t, []byte("order-123"), fields.Resolve(ctx, "orders", nil, logger))
	cancel()
	require.ErrorIs(t, ctx.Err(), context.Canceled)
}

func TestPartitionKeyPrecedence(t *testing.T) {
	fields := keyfields.Fields{"orders": "orderId"}
	payload := []byte(`{"orderId":"json-key"}`)
	logger := &silentKeyLogger{}
	assert.Equal(t, []byte("json-key"), fields.Resolve(t.Context(), "orders", payload, logger))

	for _, key := range [][]byte{nil, {}, []byte("explicit-key")} {
		ctx := pubsub.WithPartitionKey(t.Context(), key)
		got := fields.Resolve(ctx, "orders", payload, logger)

		if len(key) == 0 {
			assert.Nil(t, got, "explicitly empty keys must suppress JSON fallback")
		} else {
			assert.Equal(t, key, got)
		}
	}
}

type silentKeyLogger struct{}

func (*silentKeyLogger) Debugf(string, ...any) {}
