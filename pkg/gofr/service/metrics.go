package service

import "context"

type Metrics interface {
	IncrementCounter(ctx context.Context, name string, labels ...string)
	SetGauge(name string, value float64, labels ...string)
	RecordHistogram(ctx context.Context, name string, value float64, labels ...string)
}
