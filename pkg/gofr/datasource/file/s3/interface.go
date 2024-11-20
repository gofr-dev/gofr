package s3

import (
	"context"
)

// Logger interface is used by s3 package to log information about query execution.
type Logger interface {
	Debug(args ...interface{})
	Logf(pattern string, args ...interface{})
	Errorf(pattern string, args ...interface{})
}

type Metrics interface {
	NewHistogram(name, desc string, buckets ...float64)
	RecordHistogram(ctx context.Context, name string, value float64, labels ...string)
}
