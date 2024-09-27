package eventhub

import (
	"context"
)

type Metrics interface {
	NewHistogram(name, desc string, buckets ...float64)

	IncrementCounter(ctx context.Context, name string, labels ...string)
}
