package dbresolver

import (
	"context"
)

// Metrics interface for metrics operations.
type Metrics interface {
	NewHistogram(name, description string, buckets ...float64)
	NewGauge(name, description string)
	RecordHistogram(ctx context.Context, name string, value float64, labels ...string)
	SetGauge(name string, value float64, labels ...string)
}
