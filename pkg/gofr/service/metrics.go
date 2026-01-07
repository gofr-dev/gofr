package service

import "context"

type Metrics interface {
	NewCounter(name, desc string)
	IncrementCounter(ctx context.Context, name string, labels ...string)
	RecordHistogram(ctx context.Context, name string, value float64, labels ...string)
}
