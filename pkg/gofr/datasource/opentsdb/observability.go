package opentsdb

import (
	"context"
	"fmt"
	"io"
	"regexp"
	"strings"
	"time"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
)

// QueryLog handles logging with different levels.
type QueryLog struct {
	Operation string  `json:"operation"`
	Duration  int64   `json:"duration"`
	Status    *string `json:"status"`
	Message   *string `json:"message,omitempty"`
}

var regexpSpaces = regexp.MustCompile(`\s+`)

func clean(query *string) string {
	if query == nil {
		return ""
	}

	return strings.TrimSpace(regexpSpaces.ReplaceAllString(*query, " "))
}

func (ql *QueryLog) PrettyPrint(writer io.Writer) {
	fmt.Fprintf(writer, "\u001B[38;5;8m%-32s \u001B[38;5;148m%-6s\u001B[0m %8d\u001B[38;5;8mµs\u001B[0m %-10s \u001B[0m %-48s \n",
		clean(&ql.Operation), "OPENTSDB", ql.Duration, clean(ql.Status), clean(ql.Message))
}

func sendOperationStats(logger Logger, start time.Time, operation string, status, message *string, span trace.Span) {
	duration := time.Since(start).Milliseconds()

	logger.Debug(&QueryLog{
		Operation: operation,
		Status:    status,
		Duration:  duration,
		Message:   message,
	})

	if span != nil {
		defer span.End()
		span.SetAttributes(attribute.Int64(fmt.Sprintf("opentsdb.%v.duration", operation), duration))
	}
}

func addTracer(ctx context.Context, tracer trace.Tracer, operation, typeName string) (context.Context, trace.Span) {
	if tracer != nil {
		contextWithTrace, span := tracer.Start(ctx, fmt.Sprintf("opentsdb-%v", operation))

		span.SetAttributes(
			attribute.String(fmt.Sprintf("opentsdb-%v.operation", typeName), operation),
		)

		return contextWithTrace, span
	}

	return ctx, nil
}

func (c *OpentsdbClient) addTrace(ctx context.Context, operation string) (context.Context, trace.Span) {
	return addTracer(ctx, c.tracer, operation, "Client")
}

func (aggreResp *AggregatorsResponse) addTrace(ctx context.Context, operation string) (context.Context, trace.Span) {
	return addTracer(ctx, aggreResp.tracer, operation, "AggregatorRes")
}

func (annotResp *AnnotationResponse) addTrace(ctx context.Context, operation string) (context.Context, trace.Span) {
	return addTracer(ctx, annotResp.tracer, operation, "AnnotationRes")
}

func (bulkAnnotResp *BulkAnnotatResponse) addTrace(ctx context.Context, operation string) (context.Context, trace.Span) {
	return addTracer(ctx, bulkAnnotResp.tracer, operation, "BulkAnnotatResponse")
}

func (d *QueryResponse) addTrace(ctx context.Context, operation string) (context.Context, trace.Span) {
	return addTracer(ctx, d.tracer, operation, "QueryResponse")
}

func (d *DropcachesResponse) addTrace(ctx context.Context, operation string) (context.Context, trace.Span) {
	return addTracer(ctx, d.tracer, operation, "DropcacheResponse")
}

func (sugParam *SuggestParam) addTrace(ctx context.Context, operation string) (context.Context, trace.Span) {
	return addTracer(ctx, sugParam.tracer, operation, "SuggestParam")
}

func (sugResp *SuggestResponse) addTrace(ctx context.Context, operation string) (context.Context, trace.Span) {
	return addTracer(ctx, sugResp.tracer, operation, "SuggestResponse")
}

func (qri *QueryRespItem) addTrace(ctx context.Context, operation string) (context.Context, trace.Span) {
	return addTracer(ctx, qri.tracer, operation, "QueryRespItem")
}

func (query *QueryParam) addTrace(ctx context.Context, operation string) (context.Context, trace.Span) {
	return addTracer(ctx, query.tracer, operation, "QueryParam")
}

func (query *QueryLastParam) addTrace(ctx context.Context, operation string) (context.Context, trace.Span) {
	return addTracer(ctx, query.tracer, operation, "QueryLastParam")
}

func (ql *QueryLastResponse) addTrace(ctx context.Context, operation string) (context.Context, trace.Span) {
	return addTracer(ctx, ql.tracer, operation, "QueryLastResponse")
}

func (v *VersionResponse) addTrace(ctx context.Context, operation string) (context.Context, trace.Span) {
	return addTracer(ctx, v.tracer, operation, "VersionResponse")
}

func (v *TSMetaDataResponse) addTrace(ctx context.Context, operation string) (context.Context, trace.Span) {
	return addTracer(ctx, v.tracer, operation, "TSMetaDataResponse")
}

func (putResp *PutResponse) addTrace(ctx context.Context, operation string) (context.Context, trace.Span) {
	return addTracer(ctx, putResp.tracer, operation, "PutResponse")
}

func (uidAssignResp *UIDAssignResponse) addTrace(ctx context.Context, operation string) (context.Context, trace.Span) {
	return addTracer(ctx, uidAssignResp.tracer, operation, "UIDAssignResponse")
}

func (v *UIDMetaDataResponse) addTrace(ctx context.Context, operation string) (context.Context, trace.Span) {
	return addTracer(ctx, v.tracer, operation, "UIDMetaDataResponse")
}
