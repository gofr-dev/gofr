package redis

import (
	"context"
)

// Metrics interface for tracking pubsub metrics.
type Metrics interface {
	IncrementCounter(ctx context.Context, name string, labels ...string)
}
