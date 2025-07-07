package pinecone

import (
	"context"
	"fmt"
	"time"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
	"go.opentelemetry.io/otel/trace/embedded"
)

// SpanAttributes represents common span attributes for operations.
type SpanAttributes struct {
	Index     string
	Namespace string
	Operation string
	Count     int
	Dimension int
	Metric    string
}

// OperationContext encapsulates common operation setup.
type OperationContext struct {
	ctx       context.Context
	span      trace.Span
	startTime time.Time
	operation string
}

// spanManager handles tracing and span management.
type spanManager struct {
	client *Client
}

// newSpanManager creates a new span manager.
func newSpanManager(client *Client) *spanManager {
	return &spanManager{client: client}
}

// setupOperation creates a common operation context to reduce duplication.
func (sm *spanManager) setupOperation(ctx context.Context, operation string) OperationContext {
	ctx, span := sm.startSpan(ctx, operation)

	return OperationContext{
		ctx:       ctx,
		span:      span,
		startTime: time.Now(),
		operation: operation,
	}
}

// cleanup handles common operation cleanup.
func (sm *spanManager) cleanup(opCtx OperationContext) {
	opCtx.span.End()
	sm.client.recordMetrics(opCtx.startTime, opCtx.operation)
}

// startSpan starts a new trace span with the given name.
func (sm *spanManager) startSpan(ctx context.Context, name string) (context.Context, trace.Span) {
	if sm.client.tracer != nil {
		return sm.client.tracer.Start(ctx, fmt.Sprintf("pinecone.%s", name))
	}

	return ctx, noopSpan{}
}

// setSpanAttributes sets common span attributes to reduce duplication.
func (*spanManager) setSpanAttributes(span trace.Span, attrs *SpanAttributes) {
	if attrs.Index != "" {
		span.SetAttributes(attribute.String("index", attrs.Index))
	}

	if attrs.Namespace != "" {
		span.SetAttributes(attribute.String("namespace", attrs.Namespace))
	}

	if attrs.Count > 0 {
		span.SetAttributes(attribute.Int("count", attrs.Count))
	}

	if attrs.Dimension > 0 {
		span.SetAttributes(attribute.Int("dimension", attrs.Dimension))
	}

	if attrs.Metric != "" {
		span.SetAttributes(attribute.String("metric", attrs.Metric))
	}
}

// noopSpan implements a no-op span for tracing when tracer is not available.
type noopSpan struct {
	embedded.Span
}

func (noopSpan) End(...trace.SpanEndOption)              {}
func (noopSpan) AddEvent(string, ...trace.EventOption)   {}
func (noopSpan) IsRecording() bool                       { return false }
func (noopSpan) RecordError(error, ...trace.EventOption) {}
func (noopSpan) SpanContext() trace.SpanContext          { return trace.SpanContext{} }
func (noopSpan) SetStatus(codes.Code, string)            {}
func (noopSpan) SetName(string)                          {}
func (noopSpan) SetAttributes(...attribute.KeyValue)     {}
func (noopSpan) AddLink(trace.Link)                      {}
func (noopSpan) TracerProvider() trace.TracerProvider    { return nil }
