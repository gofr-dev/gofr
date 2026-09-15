package gofr

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/trace/noop"

	"gofr.dev/pkg/gofr/config"
	"gofr.dev/pkg/gofr/container"
	"gofr.dev/pkg/gofr/logging"
	"gofr.dev/pkg/gofr/testutil"
)

func Test_App_tracerInsecure(t *testing.T) {
	tests := []struct {
		name            string
		value           string
		expectedFlag    bool
		expectedFlagSet bool
	}{
		{name: "unset defaults to plaintext", value: "", expectedFlag: true, expectedFlagSet: false},
		{name: "explicit true", value: "true", expectedFlag: true, expectedFlagSet: true},
		{name: "explicit false", value: "false", expectedFlag: false, expectedFlagSet: true},
		{name: "numeric false", value: "0", expectedFlag: false, expectedFlagSet: true},
		{name: "whitespace is trimmed", value: " false ", expectedFlag: false, expectedFlagSet: true},
		{name: "invalid value falls back to the default", value: "yes-please", expectedFlag: true, expectedFlagSet: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			app := &App{
				Config:    config.NewMockConfig(map[string]string{"TRACER_INSECURE": tt.value}),
				container: &container.Container{Logger: logging.NewMockLogger(logging.ERROR)},
			}

			insecure, set := app.tracerInsecure()

			require.Equal(t, tt.expectedFlag, insecure)
			require.Equal(t, tt.expectedFlagSet, set)
		})
	}
}

func Test_App_initTracer(t *testing.T) {
	tests := []struct {
		name          string
		configs       map[string]string
		wantRecording bool
	}{
		{
			name:          "no exporter configured",
			configs:       map[string]string{},
			wantRecording: false,
		},
		{
			name:          "otlp",
			configs:       map[string]string{"TRACE_EXPORTER": "otlp", "TRACER_URL": "collector:4317"},
			wantRecording: true,
		},
		{
			name:          "otlp over TLS",
			configs:       map[string]string{"TRACE_EXPORTER": "otlp", "TRACER_URL": "https://collector:4317"},
			wantRecording: true,
		},
		{
			name:          "jaeger on the deprecated host and port",
			configs:       map[string]string{"TRACE_EXPORTER": "jaeger", "TRACER_HOST": "localhost", "TRACER_PORT": "4317"},
			wantRecording: true,
		},
		{
			name:          "zipkin",
			configs:       map[string]string{"TRACE_EXPORTER": "zipkin", "TRACER_URL": "http://zipkin:9411/api/v2/spans"},
			wantRecording: true,
		},
		{
			name:          "gofr needs no TRACER_URL",
			configs:       map[string]string{"TRACE_EXPORTER": "gofr"},
			wantRecording: true,
		},
		{
			name:          "unsupported exporter degrades instead of crashing",
			configs:       map[string]string{"TRACE_EXPORTER": "carrier-pigeon", "TRACER_URL": "collector:4317"},
			wantRecording: false,
		},
		{
			name:          "TRACER_URL without TRACE_EXPORTER degrades",
			configs:       map[string]string{"TRACER_URL": "collector:4317"},
			wantRecording: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Cleanup(func() { otel.SetTracerProvider(noop.NewTracerProvider()) })

			app := &App{
				Config:    config.NewMockConfig(tt.configs),
				container: &container.Container{Logger: logging.NewMockLogger(logging.ERROR)},
			}

			_ = testutil.StderrOutputForFunc(app.initTracer)

			_, span := otel.Tracer("test").Start(t.Context(), "span")
			defer span.End()

			require.Equal(t, tt.wantRecording, span.IsRecording())

			// Correlation IDs come off the SpanContext, so every configuration —
			// including the no-exporter default — must hand out valid IDs. A noop
			// provider here once zeroed them across every request.
			require.True(t, span.SpanContext().TraceID().IsValid(), "TraceID must stay valid for X-Correlation-ID")
			require.True(t, span.SpanContext().SpanID().IsValid(), "SpanID must stay valid for X-Correlation-ID")

			require.NoError(t, app.shutdownTraces(t.Context()))
		})
	}
}

func Test_App_shutdownTraces_withoutInitIsANoop(t *testing.T) {
	app := &App{}

	require.NoError(t, app.shutdownTraces(t.Context()))
}

func TestParseHeaders(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected map[string]string
	}{
		{
			name:     "empty string",
			input:    "",
			expected: map[string]string{},
		},
		{
			name:  "single header",
			input: "Key=Value",
			expected: map[string]string{
				"Key": "Value",
			},
		},
		{
			name:  "multiple headers",
			input: "K1=V1,K2=V2",
			expected: map[string]string{
				"K1": "V1",
				"K2": "V2",
			},
		},
		{
			name:  "value with equals sign",
			input: "Hash=sha256=abc123,Key=value",
			expected: map[string]string{
				"Hash": "sha256=abc123",
				"Key":  "value",
			},
		},
		{
			name:  "skip invalid entries",
			input: "NoEquals,Valid=value,=EmptyKey",
			expected: map[string]string{
				"Valid": "value",
			},
		},
		{
			name:  "trim whitespace",
			input: " Key1 = Value1 , Key2 = Value2 ",
			expected: map[string]string{
				"Key1": "Value1",
				"Key2": "Value2",
			},
		},
		{
			name:  "empty key",
			input: "=Value,Valid=value",
			expected: map[string]string{
				"Valid": "value",
			},
		},
		{
			name:  "empty value",
			input: "Key=,Valid=value",
			expected: map[string]string{
				"Valid": "value",
			},
		},
		{
			name:  "base64 authorization header",
			input: "Authorization=Basic dXNlcjpwYXNz",
			expected: map[string]string{
				"Authorization": "Basic dXNlcjpwYXNz",
			},
		},
		{
			name:  "multiple headers with special characters",
			input: "X-Api-Key=abc123xyz,Authorization=Bearer token123,X-Scope-OrgID=tenant-1",
			expected: map[string]string{
				"X-Api-Key":     "abc123xyz",
				"Authorization": "Bearer token123",
				"X-Scope-OrgID": "tenant-1",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := parseHeaders(tt.input)

			require.Equal(t, tt.expected, result)
		})
	}
}

func TestApp_getTracerHeaders_WithTracerHeaders(t *testing.T) {
	tests := []struct {
		name              string
		tracerHeaders     string
		expectedHeaders   map[string]string
		expectedHeaderLen int
	}{
		{
			name:              "multiple headers",
			tracerHeaders:     "X-Api-Key=secret123,Authorization=Bearer token",
			expectedHeaderLen: 2,
			expectedHeaders: map[string]string{
				"X-Api-Key":     "secret123",
				"Authorization": "Bearer token",
			},
		},
		{
			name:              "single header",
			tracerHeaders:     "X-Honeycomb-Team=abc123",
			expectedHeaderLen: 1,
			expectedHeaders: map[string]string{
				"X-Honeycomb-Team": "abc123",
			},
		},
		{
			name:              "priority over TRACER_AUTH_KEY",
			tracerHeaders:     "X-Custom-Header=value",
			expectedHeaderLen: 1,
			expectedHeaders: map[string]string{
				"X-Custom-Header": "value",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			configData := map[string]string{
				"TRACER_HEADERS": tt.tracerHeaders,
			}

			app := &App{
				Config: config.NewMockConfig(configData),
			}

			headers := app.getTracerHeaders()

			require.Equal(t, tt.expectedHeaders, headers)
			require.Len(t, headers, tt.expectedHeaderLen)
		})
	}
}

func TestApp_getTracerHeaders_WithAuthKey(t *testing.T) {
	tests := []struct {
		name              string
		tracerAuthKey     string
		expectedHeaders   map[string]string
		expectedHeaderLen int
	}{
		{
			name:              "backward compatibility",
			tracerAuthKey:     "Bearer legacy-token",
			expectedHeaderLen: 1,
			expectedHeaders: map[string]string{
				"Authorization": "Bearer legacy-token",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			configData := map[string]string{
				"TRACER_AUTH_KEY": tt.tracerAuthKey,
			}

			app := &App{
				Config: config.NewMockConfig(configData),
			}

			headers := app.getTracerHeaders()

			require.Equal(t, tt.expectedHeaders, headers)
			require.Len(t, headers, tt.expectedHeaderLen)
		})
	}
}

func TestApp_getTracerHeaders_NoConfig(t *testing.T) {
	app := &App{
		Config: config.NewMockConfig(map[string]string{}),
	}

	headers := app.getTracerHeaders()

	require.Empty(t, headers)
}

var (
	errOtelStatus200 = errors.New("rpc error: code = Unknown desc = status 200")
	errOtelStatus204 = errors.New("rpc error: status 204")
	errOtelStatus201 = errors.New("status 201: ok")
	errOtelStatus500 = errors.New("rpc error: status 500")
)

type captureLogger struct {
	loggedErrors []string
}

func (*captureLogger) Debug(_ ...any)             {}
func (*captureLogger) Debugf(_ string, _ ...any)  {}
func (*captureLogger) Log(_ ...any)               {}
func (*captureLogger) Logf(_ string, _ ...any)    {}
func (*captureLogger) Info(_ ...any)              {}
func (*captureLogger) Infof(_ string, _ ...any)   {}
func (*captureLogger) Notice(_ ...any)            {}
func (*captureLogger) Noticef(_ string, _ ...any) {}
func (*captureLogger) Warn(_ ...any)              {}
func (*captureLogger) Warnf(_ string, _ ...any)   {}
func (l *captureLogger) Error(args ...any) {
	// otelErrorHandler passes a single string arg
	if len(args) == 1 {
		if s, ok := args[0].(string); ok {
			l.loggedErrors = append(l.loggedErrors, s)
			return
		}
	}

	l.loggedErrors = append(l.loggedErrors, "non-string error")
}
func (*captureLogger) Errorf(_ string, _ ...any)   {}
func (*captureLogger) Fatal(_ ...any)              {}
func (*captureLogger) Fatalf(_ string, _ ...any)   {}
func (*captureLogger) ChangeLevel(_ logging.Level) {}

func TestOtelErrorHandler_Ignores2xxStatusErrors(t *testing.T) {
	cl := &captureLogger{}
	h := &otelErrorHandler{logger: cl}

	h.Handle(errOtelStatus200)
	h.Handle(errOtelStatus204)
	h.Handle(errOtelStatus201)

	require.Empty(t, cl.loggedErrors)
}

func TestOtelErrorHandler_LogsNon2xxErrors(t *testing.T) {
	cl := &captureLogger{}
	h := &otelErrorHandler{logger: cl}

	h.Handle(errOtelStatus500)

	require.Len(t, cl.loggedErrors, 1)
	require.Equal(t, "rpc error: status 500", cl.loggedErrors[0])
}

func TestOtelErrorHandler_NilErrorNoop(t *testing.T) {
	cl := &captureLogger{}
	h := &otelErrorHandler{logger: cl}

	h.Handle(nil)

	require.Empty(t, cl.loggedErrors)
}

// BenchmarkSpanStart_DefaultSDK measures the cost of starting a span
// under today's default GoFr configuration: an SDK TracerProvider is
// installed (with ParentBased(TraceIDRatioBased(1.0)) sampler — see
// initTracer in otel.go) even when no TRACE_EXPORTER is configured.
// The span is built, sampled, and discarded because there's no exporter.
//
// This is the cost that 100% of GoFr users without an exporter pay today.
//
// PR-1 target: when no TRACE_EXPORTER, install noop.NewTracerProvider()
// and skip the SDK entirely. Delta vs BenchmarkSpanStart_Noop is the
// expected win.
func BenchmarkSpanStart_DefaultSDK(b *testing.B) {
	tp := sdktrace.NewTracerProvider(
		sdktrace.WithResource(resource.Empty()),
		sdktrace.WithSampler(sdktrace.ParentBased(sdktrace.TraceIDRatioBased(1.0))),
	)
	otel.SetTracerProvider(tp)

	b.Cleanup(func() {
		otel.SetTracerProvider(noop.NewTracerProvider())
	})

	tr := otel.Tracer("gofr-bench")
	ctx := context.Background()

	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		_, span := tr.Start(ctx, "GET /plaintext")
		span.End()
	}
}

// BenchmarkSpanStart_Noop measures the cost of starting a span when a
// noop TracerProvider is installed. This is the floor — what GoFr would
// pay after PR-1 lands for users without an exporter.
func BenchmarkSpanStart_Noop(b *testing.B) {
	otel.SetTracerProvider(noop.NewTracerProvider())

	tr := otel.Tracer("gofr-bench")
	ctx := context.Background()

	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		_, span := tr.Start(ctx, "GET /plaintext")
		span.End()
	}
}

// BenchmarkSpanStart_NeverSampleSDK measures the cost of starting a span
// under the actual configuration initTracer installs when no
// TRACE_EXPORTER is set: SDK provider with NeverSample. Spans get unique
// TraceID/SpanID (needed for correlation IDs) but the SDK short-circuits
// at the sampler — no attributes, events, batch processor, or exporter.
//
// Expected: slightly more than BenchmarkSpanStart_Noop (one
// nonRecordingSpan + ID generation) but far below
// BenchmarkSpanStart_DefaultSDK (full recording span allocation).
func BenchmarkSpanStart_NeverSampleSDK(b *testing.B) {
	tp := sdktrace.NewTracerProvider(
		sdktrace.WithResource(resource.Empty()),
		sdktrace.WithSampler(sdktrace.NeverSample()),
	)
	otel.SetTracerProvider(tp)

	b.Cleanup(func() {
		otel.SetTracerProvider(noop.NewTracerProvider())
	})

	tr := otel.Tracer("gofr-bench")
	ctx := context.Background()

	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		_, span := tr.Start(ctx, "GET /plaintext")
		span.End()
	}
}

// Test_App_tracerRatio pins the TRACER_RATIO fallback. An unparsable value
// resolves to 1, not to 0: prior to the exporter registry a parse failure left
// the ratio at its zero value and silently disabled tracing. Both behaviors are
// defensible, and this test exists so the next refactor picks one deliberately
// rather than by accident — see the comment on tracerRatio.
func Test_App_tracerRatio(t *testing.T) {
	tests := []struct {
		name     string
		value    string
		expected float64
	}{
		{name: "unset defaults to full sampling", value: "", expected: 1},
		{name: "explicit full sampling", value: "1", expected: 1},
		{name: "fractional ratio", value: "0.25", expected: 0.25},
		{name: "explicit zero disables sampling", value: "0", expected: 0},
		{name: "unparsable value falls back to 1, not 0", value: "10%", expected: 1},
		{name: "non-numeric value falls back to 1, not 0", value: "abc", expected: 1},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			app := &App{
				Config:    config.NewMockConfig(map[string]string{"TRACER_RATIO": tt.value}),
				container: &container.Container{Logger: logging.NewMockLogger(logging.ERROR)},
			}

			require.InDelta(t, tt.expected, app.tracerRatio(), 0)
		})
	}
}
