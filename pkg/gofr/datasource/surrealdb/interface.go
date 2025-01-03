package surrealdb

import (
	"context"
	"fmt"
	"io"
	"strings"
)

// Logger defines methods for logging debug, log, and error messages.
type Logger interface {
	// Debugf logs a formatted debug message.
	Debugf(pattern string, args ...interface{})
	// Debug logs a debug message
	Debug(args ...interface{})
	// Logf logs a formatted log message.
	Logf(pattern string, args ...interface{})
	// Errorf logs a formatted error message
	Errorf(pattern string, args ...interface{})
}

// mockLogger is an implementation of the Logger interface for testing.
type mockLogger struct{}

func (m *mockLogger) Debugf(pattern string, args ...interface{}) {}
func (m *mockLogger) Debug(args ...interface{})                  {}
func (m *mockLogger) Logf(pattern string, args ...interface{})   {}
func (m *mockLogger) Errorf(pattern string, args ...interface{}) {}

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

type QueryLog struct {
	Query      string      `json:"query"`                // The query executed.
	Duration   int64       `json:"duration"`             // Execution time in microseconds.
	Namespace  string      `json:"namespace"`            // The namespace of the query.
	Database   string      `json:"database"`             // The database the query was executed on.
	ID         interface{} `json:"id"`                   // The ID of the affected items.
	Data       interface{} `json:"data"`                 // The data affected or retrieved.
	Filter     interface{} `json:"filter,omitempty"`     // Optional filter applied to the query.
	Update     interface{} `json:"update,omitempty"`     // Optional update data for the query.
	Collection string      `json:"collection,omitempty"` // Optional collection affected.
}

// PrettyPrint outputs a formatted string representation of the QueryLog.
func (ql *QueryLog) PrettyPrint(writer io.Writer) {
	// Ensure optional fields are not nil for string formatting.
	if ql.Filter == nil {
		ql.Filter = ""
	}

	if ql.ID == nil {
		ql.ID = ""
	}

	if ql.Update == nil {
		ql.Update = ""
	}

	// Print the formatted query log details.
	fmt.Fprintf(writer, "\u001B[38;5;8m%-32s \u001B[38;5;206m%-6s\u001B[0m %8d\u001B[38;5;8mµs\u001B[0m %s\n",
		Clean(ql.Query), "SURREAL", ql.Duration,
		Clean(strings.Join([]string{ql.Collection, fmt.Sprint(ql.Filter), fmt.Sprint(ql.ID), fmt.Sprint(ql.Update)}, " ")))
}
