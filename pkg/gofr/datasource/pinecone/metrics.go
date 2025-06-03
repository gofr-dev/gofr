package pinecone

import "context"

// Metrics defines the interface for capturing metrics related to Pinecone operations.
type Metrics interface {
	// NewHistogram registers a new histogram metric with the given name, description, and buckets.
	NewHistogram(name, desc string, buckets ...float64)
	
	// NewGauge registers a new gauge metric with the given name and description.
	NewGauge(name, desc string)
	
	// RecordHistogram records a value in the specified histogram metric with optional labels.
	RecordHistogram(ctx context.Context, name string, value float64, labels ...string)
	
	// SetGauge sets the value of the specified gauge metric with optional labels.
	SetGauge(name string, value float64, labels ...string)
}