package sqs

import (
	"context"
)

// Metrics interface for recording SQS metrics.
type Metrics interface {
	IncrementCounter(ctx context.Context, name string, labels ...string)
}
