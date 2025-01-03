package scylladb

import "context"

// Metrics constains all the necessary methods to record metrics.
type Metrics interface {
	NewHistogram(name, desc string, buckets ...float64)
	RecordHistogram(ctx context.Context, name string, value float64, labels ...string)
}
