//go:build !gofr_nootlp

package gofr

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/trace/noop"

	"gofr.dev/pkg/gofr/config"
	"gofr.dev/pkg/gofr/container"
	"gofr.dev/pkg/gofr/logging"
	"gofr.dev/pkg/gofr/testutil"
)

// The tests in this file drive the otlp and jaeger exporters through initTracer,
// so they cannot pass in a binary built with -tags gofr_nootlp, which omits them.

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

// Test_initTracer_doesNotLogCredentials drives the real startup path for every
// exporter that logs its endpoint, and asserts the credentials in TRACER_URL and
// the raw TRACER_INSECURE value never reach the log.
//
// The per-helper tests live with the helpers in traces/exporters; this one is
// what proves the framework actually routes through them, which is the part a
// refactor of initTracer can silently undo.
func Test_initTracer_doesNotLogCredentials(t *testing.T) {
	const secret = "s3cret-value"

	tests := []struct {
		name     string
		exporter string
		url      string
		insecure string
		expected string
	}{
		{name: "otlp userinfo", exporter: "otlp", url: "https://user:" + secret + "@localhost:4317",
			expected: "Exporting traces to otlp at https://REDACTED@localhost:4317"},
		{name: "jaeger ignored TRACER_INSECURE", exporter: "JAEGER", url: "http://user:" + secret + "@localhost:4317",
			insecure: "true", expected: "Exporting traces to jaeger at http://REDACTED@localhost:4317"},
		{name: "otlp invalid TRACER_INSECURE", exporter: "otlp", url: "localhost:4317", insecure: secret,
			expected: "invalid TRACER_INSECURE"},
		{name: "zipkin query key", exporter: "zipkin", url: "http://localhost:2005/api/v2/spans?api-key=" + secret,
			expected: "Exporting traces to zipkin at http://localhost:2005/api/v2/spans?REDACTED"},
		{name: "zipkin schemeless query key", exporter: "zipkin", url: "localhost:9411/api/v2/spans?api-key=" + secret,
			expected: "Exporting traces to zipkin at localhost:9411/api/v2/spans?REDACTED"},
		{name: "gofr userinfo", exporter: "gofr", url: "https://user:" + secret + "@tracer.example.com/api/spans",
			expected: "Exporting traces to GoFr at https://REDACTED@tracer.example.com/api/spans"},
		{name: "secret pasted into TRACE_EXPORTER", exporter: secret, url: "localhost:4317",
			expected: "unsupported TRACE_EXPORTER: REDACTED"},
		{name: "typo in TRACE_EXPORTER is echoed", exporter: "otpl", url: "localhost:4317",
			expected: "unsupported TRACE_EXPORTER: otpl"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := map[string]string{"TRACE_EXPORTER": tt.exporter, "TRACER_URL": tt.url, "TRACER_AUTH_KEY": "Bearer x"}
			if tt.insecure != "" {
				cfg["TRACER_INSECURE"] = tt.insecure
			}

			var stderr string

			stdout := testutil.StdoutOutputForFunc(func() {
				stderr = testutil.StderrOutputForFunc(func() {
					mockContainer, _ := container.NewMockContainer(t)

					a := App{Config: config.NewMockConfig(cfg), container: mockContainer}
					a.initTracer()
				})
			})

			out := stdout + stderr

			require.Contains(t, out, tt.expected)
			require.NotContains(t, out, secret)
		})
	}
}

func Test_initTracer_invalidConfig(t *testing.T) {
	createMockConfig := func(traceExporter, url, authKey string) config.Config {
		return config.NewMockConfig(map[string]string{
			"TRACE_EXPORTER":  traceExporter,
			"TRACER_URL":      url,
			"TRACER_AUTH_KEY": authKey,
		})
	}
	mockConfig1 := createMockConfig("abc", "https://tracer-service.dev", "")
	mockConfig2 := createMockConfig("", "https://tracer-service.dev", "")
	mockConfig3 := createMockConfig("otlp", "", "")

	testErr := []struct {
		desc               string
		config             config.Config
		expectedLogMessage string
	}{
		{"unsupported trace_exporter", mockConfig1, "unsupported TRACE_EXPORTER: abc; tracing is disabled"},
		{"missing trace_exporter", mockConfig2, "missing TRACE_EXPORTER config, should be provided with TRACER_URL to enable tracing"},
		{"miss tracer_url ", mockConfig3,
			"missing TRACER_URL config, should be provided with TRACE_EXPORTER to enable tracing"},
	}

	for i, tc := range testErr {
		logMessage := testutil.StderrOutputForFunc(func() {
			mockContainer, _ := container.NewMockContainer(t)

			a := App{
				Config:    tc.config,
				container: mockContainer,
			}
			a.initTracer()
		})

		assert.Contains(t, logMessage, tc.expectedLogMessage, "TEST[%d], Failed.\n%s", i, tc.desc)
	}
}
