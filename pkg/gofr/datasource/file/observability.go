package file

import (
	"context"
	"time"

	"gofr.dev/pkg/gofr/datasource"
)

const (
	// AppFileStats is the single metric name for all file operations across providers.
	AppFileStats = "app_file_stats"
)

// Operation constants for standardization across all file providers.
const (
	OpConnect   = "CONNECT"
	OpCreate    = "CREATE"
	OpOpen      = "OPEN"
	OpOpenFile  = "OPEN_FILE"
	OpRemove    = "REMOVE"
	OpRename    = "RENAME"
	OpMkdir     = "MKDIR"
	OpMkdirAll  = "MKDIR_ALL"
	OpRemoveAll = "REMOVE_ALL"
	OpReadDir   = "READ_DIR"
	OpStat      = "STAT"
	OpChDir     = "CHDIR"
	OpGetwd     = "GETWD"
	OpRead      = "READ"
	OpReadAt    = "READ_AT"
	OpWrite     = "WRITE"
	OpWriteAt   = "WRITE_AT"
	OpSeek      = "SEEK"
	OpClose     = "CLOSE"
)

// StorageMetrics interface that all storage providers should use.
type StorageMetrics interface {
	// NewHistogram creates a new histogram with the given name, description, and buckets.
	NewHistogram(name, desc string, buckets ...float64)

	// RecordHistogram records a value in the histogram with the given name and labels.
	RecordHistogram(ctx context.Context, name string, value float64, labels ...string)
}

// DefaultHistogramBuckets returns the standard bucket sizes for file operations.
func DefaultHistogramBuckets() []float64 {
	return []float64{0.1, 1, 10, 100, 1000}
}

// OperationObservability contains all parameters needed for logging and metrics collection.
type OperationObservability struct {
	Context   context.Context
	Logger    datasource.Logger
	Metrics   StorageMetrics
	Operation string
	Location  string
	Provider  string
	StartTime time.Time
	Status    *string
	Message   *string
}

// ObserveOperation is a helper function that handles both logging and metrics recording.
func ObserveOperation(params *OperationObservability) {
	duration := time.Since(params.StartTime).Microseconds()

	log := &OperationLog{
		Operation: params.Operation,
		Duration:  duration,
		Status:    params.Status,
		Location:  params.Location,
		Message:   params.Message,
		Provider:  params.Provider,
	}

	if params.Logger != nil {
		params.Logger.Debug(log)
	}

	if params.Metrics != nil {
		params.Metrics.RecordHistogram(
			params.Context,
			AppFileStats,
			float64(duration),
			"type", params.Operation,
			"status", cleanString(params.Status),
			"provider", params.Provider,
		)
	}
}
