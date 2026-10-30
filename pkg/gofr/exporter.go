package gofr

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"slices"
	"strconv"
	"strings"
	"time"

	"go.opentelemetry.io/otel/attribute"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"

	"gofr.dev/pkg/gofr/logging"
	"gofr.dev/pkg/gofr/traces/exporters"
)

// errUnexpectedStatusCode is returned when an unexpected status code is received from the remote endpoint.
var errUnexpectedStatusCode = errors.New("unexpected response status code")

// redactedCredential replaces a credential from the endpoint in an error's text.
const redactedCredential = "REDACTED"

// emptyAttributeValue is the value exported for an attribute that carries no data.
const emptyAttributeValue = "invalid"

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
		return fmt.Errorf("failed to create HTTP request, error: %w", redactRequestError(err, e.endpoint))
	}

	req.Header.Set("Content-Type", contentTypeJSON)

	client := &http.Client{}

	resp, err := client.Do(req)
	if err != nil {
		err = redactRequestError(err, e.endpoint)
		logger.Errorf("failed to create spans, error: %v", err)

		return err
	}
	defer resp.Body.Close()

	// Accept any 2xx: the hosted tracer returns 201, but Zipkin-compatible collectors (a valid
	// TRACER_URL target) return 202 Accepted.
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		// Log as well as return: the batch processor swallows the returned error, so without this a
		// misconfigured or rejecting endpoint drops every trace with no signal to the operator.
		err := fmt.Errorf("failed to post spans on '%v', %w: '%d'", exporters.RedactURL(e.endpoint), errUnexpectedStatusCode, resp.StatusCode)
		logger.Errorf("%v", err)

		return err
	}

	return nil
}

// redactRequestError returns a failed request's error without the endpoint's credentials. TRACER_URL can carry
// them in its userinfo or query, and the *url.Error from http.NewRequest or http.Client names the URL with at most
// the password masked. The *url.Error is kept, so errors.As and its Timeout method still work, with the URL
// redacted and the cause passed through redactCause.
func redactRequestError(err error, endpoint string) error {
	var urlErr *url.Error
	if errors.As(err, &urlErr) {
		return &url.Error{Op: urlErr.Op, URL: exporters.RedactURL(endpoint), Err: redactCause(urlErr.Err, endpoint)}
	}

	return redactCause(err, endpoint)
}

// redactCause hides any credential from the endpoint that a request error's cause repeats: a schemeless
// TRACER_URL such as user:pass@host fails with `unsupported protocol scheme "user"`. The returned error still
// unwraps to the cause, so errors.Is keeps matching context.Canceled, DeadlineExceeded and the like.
func redactCause(err error, endpoint string) error {
	if err == nil {
		return nil
	}

	msg := exporters.RedactMessage(err.Error(), endpoint)

	for _, secret := range endpointCredentials(endpoint) {
		msg = strings.ReplaceAll(msg, secret, redactedCredential)
	}

	if msg == err.Error() {
		return err
	}

	return &redactedError{msg: msg, err: err}
}

// endpointCredentials returns the parts of an endpoint that can hold a credential: the userinfo and the query,
// longest first so that a credential containing another is replaced whole. Like exporters.RedactURL, an endpoint
// that does not parse with a host is split on its raw text and over-redacted rather than risk a leak.
func endpointCredentials(endpoint string) []string {
	var userinfo, query string

	if u, err := url.Parse(endpoint); err == nil && u.Host != "" {
		password, _ := u.User.Password()
		userinfo = u.User.Username() + ":" + password
		query = u.RawQuery
	} else {
		raw, _, _ := strings.Cut(endpoint, "#")
		raw, query, _ = strings.Cut(raw, "?")

		if i := strings.LastIndex(raw, "@"); i >= 0 {
			userinfo = raw[:i]
			if _, rest, found := strings.Cut(userinfo, "://"); found {
				userinfo = rest
			}
		}
	}

	credentials := strings.Split(userinfo, ":")

	if query != "" {
		credentials = append(credentials, query)

		for pair := range strings.SplitSeq(query, "&") {
			_, value, _ := strings.Cut(pair, "=")
			unescaped, _ := url.QueryUnescape(value)
			credentials = append(credentials, value, unescaped)
		}
	}

	credentials = slices.DeleteFunc(credentials, func(s string) bool { return s == "" })
	slices.SortFunc(credentials, func(a, b string) int { return len(b) - len(a) })

	return credentials
}

// redactedError is a cause whose text had a credential removed. Unwrap returns the original cause.
type redactedError struct {
	msg string
	err error
}

func (e *redactedError) Error() string { return e.msg }

func (e *redactedError) Unwrap() error { return e.err }

// convertSpans converts OpenTelemetry spans to the format expected by the exporter.
func convertSpans(spans []sdktrace.ReadOnlySpan) []Span {
	convertedSpans := make([]Span, 0, len(spans))

	for _, s := range spans {
		convertedSpan := Span{
			TraceID:   s.SpanContext().TraceID().String(),
			ID:        s.SpanContext().SpanID().String(),
			Name:      s.Name(),
			Timestamp: s.StartTime().UnixNano() / int64(time.Microsecond),
			Duration:  s.EndTime().Sub(s.StartTime()).Nanoseconds() / int64(time.Microsecond),
			Tags:      make(map[string]string, len(s.Attributes())+len(s.Resource().Attributes())),
		}

		// A root span has no parent; emit parentId only when the parent span ID is valid, so the
		// field is omitted rather than sent as an all-zero ID that a backend reads as a dangling
		// parent (which can prevent it from assembling the trace).
		if parent := s.Parent(); parent.SpanID().IsValid() {
			convertedSpan.ParentID = parent.SpanID().String()
		}

		for _, kv := range s.Attributes() {
			k, v := attributeToStringPair(kv)
			convertedSpan.Tags[k] = v
		}

		for _, kv := range s.Resource().Attributes() {
			k, v := attributeToStringPair(kv)
			convertedSpan.Tags[k] = v
		}

		// Each span carries its own service name, not the batch's first span's.
		convertedSpan.LocalEndpoint = map[string]string{"serviceName": convertedSpan.Tags["service.name"]}

		convertedSpans = append(convertedSpans, convertedSpan)
	}

	return convertedSpans
}

func attributeToStringPair(kv attribute.KeyValue) (key, value string) {
	switch kv.Value.Type() {
	// For slice attributes, serialize as JSON list string. AsInterface returns the
	// typed slice ([]bool, []int64, []float64 or []string) for these four types.
	case attribute.BOOLSLICE, attribute.INT64SLICE, attribute.FLOAT64SLICE, attribute.STRINGSLICE:
		data, _ := json.Marshal(kv.Value.AsInterface())
		return string(kv.Key), string(data)
	case attribute.BOOL:
		return string(kv.Key), strconv.FormatBool(kv.Value.AsBool())
	case attribute.INT64:
		return string(kv.Key), strconv.FormatInt(kv.Value.AsInt64(), 10)
	case attribute.FLOAT64:
		return string(kv.Key), strconv.FormatFloat(kv.Value.AsFloat64(), 'f', -1, 64)
	case attribute.STRING:
		return string(kv.Key), kv.Value.AsString()
	case attribute.BYTESLICE, attribute.SLICE, attribute.MAP:
		// No JSON form of our own; use the value's own string encoding.
		return string(kv.Key), kv.Value.String()
	case attribute.EMPTY:
		return string(kv.Key), emptyAttributeValue
	default:
		return string(kv.Key), kv.Value.String()
	}
}
