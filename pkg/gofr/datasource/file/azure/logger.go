package azure

import (
	"context"
	"time"
)

// FileLog represents the structure for logging file operations.
type FileLog struct {
	Operation string
	Location  string
	Status    *string
	Message   *string
}

// sendOperationStats sends operation statistics to metrics if available.
func (f *File) sendOperationStats(log *FileLog, startTime time.Time) {
	if f.metrics != nil {
		duration := time.Since(startTime).Seconds()
		f.metrics.RecordHistogram(f.ctx, "azure_file_operation_duration", duration, log.Operation, log.Location)
	}
}

// sendOperationStats sends operation statistics to metrics if available.
func (f *FileSystem) sendOperationStats(log *FileLog, startTime time.Time) {
	if f.metrics != nil {
		duration := time.Since(startTime).Seconds()
		f.metrics.RecordHistogram(context.Background(), "azure_filesystem_operation_duration", duration, log.Operation, log.Location)
	}
}
