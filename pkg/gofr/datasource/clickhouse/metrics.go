package clickhouse

import "context"

type Metrics interface {
	NewHistogram(name, desc string, buckets ...float64)
	NewGauge(name, desc string)

	RecordHistogram(ctx context.Context, name string, value float64, labels ...string)
	SetGauge(name string, value float64, labels ...string)
}
