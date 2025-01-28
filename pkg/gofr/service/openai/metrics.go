package openai

import "context"

type Metrics interface {
	NewHistogram(name, desc string, buckets ...float64)
	NewCounter(name, desc string, labels ...string)
	NewCounterVec(name, desc string, labels ...string)

	RecordHistogram(ctx context.Context, name string, value float64, labels ...string)
	RecordRequestCount(ctx context.Context, name string, labels ...string)
	RecordTokenUsage(ctx context.Context, name string, promptTokens, completionTokens int, labels ...string)
}
