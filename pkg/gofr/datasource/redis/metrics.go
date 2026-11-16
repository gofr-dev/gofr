package redis

import "context"

type Metrics interface {
	RecordHistogram(ctx context.Context, name string, value float64, labels ...string)
	IncrementCounter(ctx context.Context, name string, labels ...string)
}
