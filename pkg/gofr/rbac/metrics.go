package rbac

import "context"

// Metrics defines methods for recording metrics that RBAC uses.
// This interface allows RBAC to record authorization metrics (e.g., authorization decision latency).
type Metrics interface {
	// NewHistogram creates a new histogram metric with specified buckets.
	NewHistogram(name, desc string, buckets ...float64)

	// RecordHistogram records a value in a histogram.
	RecordHistogram(ctx context.Context, name string, value float64, labels ...string)
}

