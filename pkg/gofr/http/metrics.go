// Package http provides a set of utilities for handling HTTP requests and responses within the GoFr framework.
package http

import "context"

// Metrics represents an interface for registering the default metrics in GoFr framework.
type Metrics interface {
	IncrementCounter(ctx context.Context, name string, labels ...string)
	DeltaUpDownCounter(ctx context.Context, name string, value float64, labels ...string)
	RecordHistogram(ctx context.Context, name string, value float64, labels ...string)
	SetGauge(name string, value float64)
}
