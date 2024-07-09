package gofr

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"go.opentelemetry.io/otel/attribute"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"

	"gofr.dev/pkg/gofr/logging"
)

// errUnexpectedStatusCode is returned when an unexpected status code is received from the remote endpoint.
var errUnexpectedStatusCode = errors.New("unexpected response status code")

// Exporter is responsible for exporting spans to a remote endpoint.
type Exporter struct {
	endpoint string         // The endpoint to which spans will be exported.
	logger   logging.Logger // Logger for logging errors and other messages.
}

// NewExporter creates a new Exporter instance with a custom endpoint and logger.
func NewExporter(endpoint string, logger logging.Logger) *Exporter {
	return &Exporter{
		endpoint: endpoint,
		logger:   logger,
	}
}

// Span represents a span that will be exported.
type Span struct {
	TraceID       string            `json:"traceId"`            // Trace ID of the span.
	ID            string            `json:"id"`                 // ID of the span.
	ParentID      string            `json:"parentId,omitempty"` // Parent ID of the span.
	Name          string            `json:"name"`               // Name of the span.
	Timestamp     int64             `json:"timestamp"`          // Timestamp of the span.
	Duration      int64             `json:"duration"`           // Duration of the span.
	Tags          map[string]string `json:"tags,omitempty"`     // Tags associated with the span.
	LocalEndpoint map[string]string `json:"localEndpoint"`      // Local endpoint of the span.
}

// ExportSpans exports spans to the configured remote endpoint.
func (e *Exporter) ExportSpans(ctx context.Context, spans []sdktrace.ReadOnlySpan) error {
	return e.processSpans(ctx, e.logger, spans)
}

// Shutdown shuts down the exporter.
func (*Exporter) Shutdown(context.Context) error {
	return nil
}

// processSpans processes spans and exports them to the configured endpoint.
func (e *Exporter) processSpans(ctx context.Context, logger logging.Logger, spans []sdktrace.ReadOnlySpan) error {
	if len(spans) == 0 {
		return nil
	}

	convertedSpans := convertSpans(spans)

	payload, err := json.Marshal(convertedSpans)
	if err != nil {
		return fmt.Errorf("failed to marshal spans, error: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, e.endpoint, bytes.NewBuffer(payload))
	if err != nil {
		return fmt.Errorf("failed to create HTTP request, error: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{}

	resp, err := client.Do(req)
	if err != nil {
		logger.Errorf("failed to create spans, error: %v", err)
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		return fmt.Errorf("failed to post spans on '%v', %w: '%d'", e.endpoint, errUnexpectedStatusCode, resp.StatusCode)
	}

	return nil
}

// convertSpans converts OpenTelemetry spans to the format expected by the exporter.
func convertSpans(spans []sdktrace.ReadOnlySpan) []Span {
	convertedSpans := make([]Span, 0, len(spans))

	for i, s := range spans {
		convertedSpan := Span{
			TraceID:   s.SpanContext().TraceID().String(),
			ID:        s.SpanContext().SpanID().String(),
			ParentID:  s.Parent().SpanID().String(),
			Name:      s.Name(),
			Timestamp: s.StartTime().UnixNano() / int64(time.Microsecond),
			Duration:  s.EndTime().Sub(s.StartTime()).Nanoseconds() / int64(time.Microsecond),
			Tags:      make(map[string]string, len(s.Attributes())+len(s.Resource().Attributes())),
		}

		for _, kv := range s.Attributes() {
			k, v := attributeToStringPair(kv)
			convertedSpan.Tags[k] = v
		}

		for _, kv := range s.Resource().Attributes() {
			k, v := attributeToStringPair(kv)
			convertedSpan.Tags[k] = v
		}

		convertedSpans = append(convertedSpans, convertedSpan)

		convertedSpans[i].LocalEndpoint = map[string]string{"serviceName": convertedSpans[0].Tags["service.name"]}
	}

	return convertedSpans
}

func attributeToStringPair(kv attribute.KeyValue) (key, value string) {
	switch kv.Value.Type() {
	// For slice attributes, serialize as JSON list string.
	case attribute.BOOLSLICE:
		data, _ := json.Marshal(kv.Value.AsBoolSlice())
		return string(kv.Key), string(data)
	case attribute.INT64SLICE:
		data, _ := json.Marshal(kv.Value.AsInt64Slice())
		return string(kv.Key), string(data)
	case attribute.FLOAT64SLICE:
		data, _ := json.Marshal(kv.Value.AsFloat64Slice())
		return string(kv.Key), string(data)
	case attribute.STRINGSLICE:
		data, _ := json.Marshal(kv.Value.AsStringSlice())
		return string(kv.Key), string(data)
	case attribute.BOOL:
		return string(kv.Key), strconv.FormatBool(kv.Value.AsBool())
	case attribute.INT64:
		return string(kv.Key), strconv.FormatInt(kv.Value.AsInt64(), 10)
	case attribute.FLOAT64:
		return string(kv.Key), strconv.FormatFloat(kv.Value.AsFloat64(), 'f', -1, 64)
	case attribute.STRING:
		return string(kv.Key), kv.Value.AsString()
	case attribute.INVALID:
		return string(kv.Key), "invalid"
	default:
		return string(kv.Key), kv.Value.Emit()
	}
}
