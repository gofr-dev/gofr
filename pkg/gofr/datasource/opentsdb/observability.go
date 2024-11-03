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

func addTracer(ctx context.Context, tracer trace.Tracer, operation, typeName string) trace.Span {
	if tracer != nil {
		_, span := tracer.Start(ctx, fmt.Sprintf("opentsdb-%s", operation))

		span.SetAttributes(
			attribute.String(fmt.Sprintf("opentsdb-%s.operation", typeName), operation),
		)

		return span
	}

	return nil
}

func (c *Client) addTrace(ctx context.Context, operation string) trace.Span {
	return addTracer(ctx, c.tracer, operation, "Client")
}

func (aggreResp *AggregatorsResponse) addTrace(ctx context.Context, operation string) trace.Span {
	return addTracer(ctx, aggreResp.tracer, operation, "AggregatorRes")
}

func (annotResp *AnnotationResponse) addTrace(ctx context.Context, operation string) trace.Span {
	return addTracer(ctx, annotResp.tracer, operation, "AnnotationRes")
}

func (queryResp *QueryResponse) addTrace(ctx context.Context, operation string) trace.Span {
	return addTracer(ctx, queryResp.tracer, operation, "QueryResponse")
}

func (qri *QueryRespItem) addTrace(ctx context.Context, operation string) trace.Span {
	return addTracer(ctx, qri.tracer, operation, "QueryRespItem")
}

func (query *QueryParam) addTrace(ctx context.Context, operation string) trace.Span {
	return addTracer(ctx, query.tracer, operation, "QueryParam")
}

func (query *QueryLastParam) addTrace(ctx context.Context, operation string) trace.Span {
	return addTracer(ctx, query.tracer, operation, "QueryLastParam")
}

func (queryLastResp *QueryLastResponse) addTrace(ctx context.Context, operation string) trace.Span {
	return addTracer(ctx, queryLastResp.tracer, operation, "QueryLastResponse")
}

func (verResp *VersionResponse) addTrace(ctx context.Context, operation string) trace.Span {
	return addTracer(ctx, verResp.tracer, operation, "VersionResponse")
}

func (putResp *PutResponse) addTrace(ctx context.Context, operation string) trace.Span {
	return addTracer(ctx, putResp.tracer, operation, "PutResponse")
}
