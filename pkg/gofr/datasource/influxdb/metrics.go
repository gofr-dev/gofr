package influxdb

import "context"

// Metrics defines the interface for capturing metrics.
type Metrics interface {
	NewHistogram(name, desc string, buckets ...float64)
	RecordHistogram(ctx context.Context, name string, value float64, labels ...string)
}
