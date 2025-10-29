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
	OpConnect       = "CONNECT"
	OpCreate        = "CREATE FILE"
	OpOpen          = "OPEN FILE"
	OpRead          = "READ"
	OpWrite         = "WRITE"
	OpClose         = "CLOSE"
	OpSeek          = "SEEK"
	OpReadAt        = "READ_AT"
	OpWriteAt       = "WRITE_AT"
	OpRemove        = "REMOVE FILE"
	OpRename        = "RENAME"
	OpMkdir         = "MKDIR"
	OpMkdirAll      = "MKDIRALL"
	OpRemoveAll     = "REMOVEALL"
	OpReadDir       = "READDIR"
	OpStat          = "STAT"
	OpChDir         = "CHDIR"
	OpGetwd         = "GETWD"
	OpReadAll       = "READALL"
	OpJSONReader    = "JSON READER"
	OpTextCSVReader = "TEXT/CSV READER"

	// FileInfo operations.
	OpGetName  = "GET NAME"
	OpFileSize = "FILE/DIR SIZE"
	OpLastMod  = "LAST MODIFIED"
	OpMode     = "MODE"
	OpIsDir    = "IS DIR"
	OpSys      = "SYS"
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

	params.Logger.Debug(log)

	params.Metrics.RecordHistogram(
		params.Context,
		AppFileStats,
		float64(duration),
		"type", params.Operation,
		"status", cleanString(params.Status),
		"provider", params.Provider,
	)
}
