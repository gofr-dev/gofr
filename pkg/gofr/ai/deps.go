package ai

import (
	"context"

	"go.opentelemetry.io/otel/trace"
)

// Metrics is the subset of the framework metrics manager the instrumentation records into.
type Metrics interface {
	IncrementCounter(ctx context.Context, name string, labels ...string)
	RecordHistogram(ctx context.Context, name string, value float64, labels ...string)
}

// MetricRegistrar registers the LLM metric definitions. The framework metrics manager satisfies it.
type MetricRegistrar interface {
	NewCounter(name, desc string)
	NewHistogram(name, desc string, buckets ...float64)
}

// Logger is the subset of the framework logger the instrumentation writes to.
type Logger interface {
	Debug(args ...any)
	Error(args ...any)
}

// Deps carries the instrumentation and tool dependencies for a model. Every field is optional; a
// nil field disables that concern. It is a struct so it can grow without breaking callers.
type Deps struct {
	Metrics Metrics
	Tracer  trace.Tracer
	Logger  Logger
	Tools   Tools
}

// CallInfo identifies a single model call for instrumentation.
type CallInfo struct {
	Deps     Deps
	Provider string
	Model    string
	Op       string
}
