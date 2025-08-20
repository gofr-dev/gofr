package couchbase

import "context"

// Metrics is an interface for collecting metrics.
type Metrics interface {
	// NewHistogram creates a new histogram metric.
	NewHistogram(name, desc string, buckets ...float64)
	// RecordHistogram records a value in a histogram metric.
	RecordHistogram(ctx context.Context, name string, value float64, labels ...string)
}
