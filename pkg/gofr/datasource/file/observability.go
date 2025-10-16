package file

import (
	"context"
	"time"

	"gofr.dev/pkg/gofr/datasource"
)

const (
	// AppFileStats is the single metric name for all file operations across providers
	AppFileStats = "app_file_stats"
)

// StorageMetrics interface that all storage providers should use
type StorageMetrics interface {
	// NewHistogram creates a new histogram with the given name, description, and buckets
	NewHistogram(name, desc string, buckets ...float64)

	// RecordHistogram records a value in the histogram with the given name and labels
	RecordHistogram(ctx context.Context, name string, value float64, labels ...string)
}

// DefaultHistogramBuckets returns the standard bucket sizes for file operations
func DefaultHistogramBuckets() []float64 {
	return []float64{0.1, 1, 10, 100, 1000}
}

// LogFileOperation is a helper function that handles both logging and metrics recording
func LogFileOperation(
	ctx context.Context,
	logger datasource.Logger,
	metrics StorageMetrics,
	operation string,
	location string,
	provider string,
	startTime time.Time,
	status *string,
	message *string,
) {
	duration := time.Since(startTime).Microseconds()

	log := &FileOperationLog{
		Operation: operation,
		Duration:  duration,
		Status:    status,
		Location:  location,
		Message:   message,
		Provider:  provider,
	}

	logger.Debug(log)

	metrics.RecordHistogram(
		ctx,
		AppFileStats,
		float64(duration),
		"type", operation,
		"status", CleanString(status),
		"provider", provider,
	)
}
