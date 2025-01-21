package surrealdb

import (
	"context"
)

// Logger defines methods for logging debug, log, and error messages.
type Logger interface {
	// Debugf logs a formatted debug message.
	Debugf(pattern string, args ...any)
	// Debug logs a debug message
	Debug(args ...any)
	// Logf logs a formatted log message.
	Logf(pattern string, args ...any)
	// Errorf logs a formatted error message
	Errorf(pattern string, args ...any)
}

// Connection defines methods for interacting with a database connection.
type Connection interface {
	// Connect establishes a connection to the database.
	Connect() error
	// Close terminates the database connection.
	Close() error
	// Send executes a database operation with the given method and parameters.
	Send(res any, method string, params ...any) error
	// Use sets the active namespace and database for the connection.
	Use(namespace string, database string) error
	// Let sets a session variable in the connection.
	Let(key string, value any) error
	// Unset removes a session variable from the connection.
	Unset(key string) error
}

// Metrics provides methods to record and manage application metrics.
type Metrics interface {
	// NewCounter creates a new counter metric.
	NewCounter(name, desc string)
	// NewUpDownCounter creates a new up-down counter metric.
	NewUpDownCounter(name, desc string)
	// NewHistogram creates a new histogram metric with specified buckets.
	NewHistogram(name, desc string, buckets ...float64)
	// NewGauge creates a new gauge metric.
	NewGauge(name, desc string)

	// IncrementCounter increments a counter by 1.
	IncrementCounter(ctx context.Context, name string, labels ...string)
	// DeltaUpDownCounter updates a delta for an up-down counter.
	DeltaUpDownCounter(ctx context.Context, name string, value float64, labels ...string)
	// RecordHistogram records a value in a histogram.
	RecordHistogram(ctx context.Context, name string, value float64, labels ...string)
	// SetGauge sets the value of a gauge.
	SetGauge(name string, value float64, labels ...string)
}
