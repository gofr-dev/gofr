package pubsub

import (
	"context"

	"gofr.dev/pkg/gofr/datasource/pubsub/internal/keyfields"
)

// WithPartitionKey returns a derived context with a per-message partition key.
// Kafka and Azure EventHub honor this key ahead of PUBSUB_KEY_FIELDS; other
// backends ignore it. The payload can use any serialization format.
// A nil or empty key explicitly selects unkeyed publication, overriding field
// extraction. The key is copied, so callers may reuse its backing buffer.
// Use UTF-8 keys for EventHub, whose PartitionKey is a string.
func WithPartitionKey(ctx context.Context, key []byte) context.Context {
	return keyfields.WithKey(ctx, key)
}
