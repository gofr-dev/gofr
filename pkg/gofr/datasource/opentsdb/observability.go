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

func sendOperationStats(
	ctx context.Context,
	logger Logger,
	metrics Metrics,
	host string,
	start time.Time,
	operation string,
	status, message *string,
	span trace.Span,
) {
	duration := time.Since(start)

	logger.Debug(&QueryLog{
		Operation: operation,
		Status:    status,
		Duration:  duration.Milliseconds(),
		Message:   message,
	})

	if span != nil {
		span.SetAttributes(attribute.Int64(fmt.Sprintf("opentsdb.%v.duration", operation), duration.Microseconds()))
		span.End()
	}

	if metrics != nil {
		statusLabel := ""
		if status != nil {
			statusLabel = *status
		}

		labels := []string{"operation", operation}
		if statusLabel != "" {
			labels = append(labels, "status", statusLabel)
		}

		if host != "" {
			labels = append(labels, "host", host)
		}

		metrics.RecordHistogram(ctx, opentsdbOperationDurationName, float64(duration.Milliseconds()), labels...)
		metrics.IncrementCounter(ctx, opentsdbOperationTotalName, labels...)
	}
}

func addTracer(ctx context.Context, tracer trace.Tracer, operation, typeName string) trace.Span {
	if tracer == nil {
		return nil
	}

	_, span := tracer.Start(ctx, fmt.Sprintf("opentsdb-%s", operation))

	span.SetAttributes(
		attribute.String(fmt.Sprintf("opentsdb-%s.operation", typeName), operation),
	)

	return span
}

func (c *Client) addTrace(ctx context.Context, operation string) trace.Span {
	return addTracer(ctx, c.tracer, operation, "Client")
}

func (*AggregatorsResponse) addTrace(ctx context.Context, tracer trace.Tracer, operation string) trace.Span {
	return addTracer(ctx, tracer, operation, "AggregatorRes")
}

func (*AnnotationResponse) addTrace(ctx context.Context, tracer trace.Tracer, operation string) trace.Span {
	return addTracer(ctx, tracer, operation, "AnnotationRes")
}

func (*QueryResponse) addTrace(ctx context.Context, tracer trace.Tracer, operation string) trace.Span {
	return addTracer(ctx, tracer, operation, "QueryResponse")
}

func (*QueryRespItem) addTrace(ctx context.Context, tracer trace.Tracer, operation string) trace.Span {
	return addTracer(ctx, tracer, operation, "QueryRespItem")
}

func (*QueryParam) addTrace(ctx context.Context, tracer trace.Tracer, operation string) trace.Span {
	return addTracer(ctx, tracer, operation, "QueryParam")
}

func (*QueryLastParam) addTrace(ctx context.Context, tracer trace.Tracer, operation string) trace.Span {
	return addTracer(ctx, tracer, operation, "QueryLastParam")
}

func (*QueryLastResponse) addTrace(ctx context.Context, tracer trace.Tracer, operation string) trace.Span {
	return addTracer(ctx, tracer, operation, "QueryLastResponse")
}

func (*VersionResponse) addTrace(ctx context.Context, tracer trace.Tracer, operation string) trace.Span {
	return addTracer(ctx, tracer, operation, "VersionResponse")
}

func (*PutResponse) addTrace(ctx context.Context, tracer trace.Tracer, operation string) trace.Span {
	return addTracer(ctx, tracer, operation, "PutResponse")
}
