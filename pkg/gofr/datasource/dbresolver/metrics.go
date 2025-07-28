package dbresolver

import (
	"context"
)

// Metrics defines the metrics interface for dbresolver
type Metrics interface {
	NewHistogram(name, desc string, buckets ...float64)
	RecordHistogram(ctx context.Context, name string, value float64, labels ...string)
}
