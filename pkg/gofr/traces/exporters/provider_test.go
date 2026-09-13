package exporters

import (
	"context"
	"strings"
	"testing"

	"go.opentelemetry.io/otel/attribute"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"

	"gofr.dev/pkg/gofr/logging"
	"gofr.dev/pkg/gofr/testutil"
)

// Whether a provider records spans is the observable difference between the
// NeverSample fallback and a provider that actually exports.
func Test_Build_degradesToNeverSample(t *testing.T) {
	Register("test-build-ok", stubBuilder)
	Register("test-build-fails", failingBuilder)

	tests := []struct {
		name          string
		cfg           Config
		wantRecording bool
		wantLog       string
	}{
		{
			name:          "no exporter configured",
			cfg:           Config{AppName: "app", Ratio: 1},
			wantRecording: false,
		},
		{
			name:          "unknown exporter name",
			cfg:           Config{AppName: "app", Exporter: "carrier-pigeon", Ratio: 1},
			wantRecording: false,
			wantLog:       "unsupported TRACE_EXPORTER",
		},
		{
			name:          "known external exporter without its blank import",
			cfg:           Config{AppName: "app", Exporter: "gcp", Ratio: 1},
			wantRecording: false,
			wantLog:       "gofr.dev/pkg/gofr/traces/exporters/gcp",
		},
		{
			name:          "builder returns an error",
			cfg:           Config{AppName: "app", Exporter: "test-build-fails", Ratio: 1},
			wantRecording: false,
			wantLog:       "builder failed",
		},
		{
			name:          "registered exporter builds a sampling provider",
			cfg:           Config{AppName: "app", Exporter: "test-build-ok", Ratio: 1},
			wantRecording: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var recording bool

			// Every degrade path logs at error level, which the framework logger
			// writes to stderr.
			out := testutil.StderrOutputForFunc(func() {
				shutdown, tp := Build(t.Context(), &tt.cfg, logging.NewMockLogger(logging.DEBUG))

				_, span := tp.Tracer("test").Start(t.Context(), "span")
				recording = span.IsRecording()

				span.End()

				if err := shutdown(t.Context()); err != nil {
					t.Errorf("shutdown returned an error: %v", err)
				}

				// Shutdown is public API and reachable twice (manual call plus the
				// signal handler), so it must stay a no-op after the first call.
				if err := shutdown(t.Context()); err != nil {
					t.Errorf("second shutdown returned an error: %v", err)
				}
			})

			if recording != tt.wantRecording {
				t.Errorf("span recording = %v, want %v", recording, tt.wantRecording)
			}

			if tt.wantLog != "" && !strings.Contains(out, tt.wantLog) {
				t.Errorf("expected log to mention %q, got: %q", tt.wantLog, out)
			}
		})
	}
}

func Test_buildResource(t *testing.T) {
	t.Setenv("OTEL_RESOURCE_ATTRIBUTES", "deployment.environment=staging")

	Register("test-resource", stubBuilder)
	RegisterResourceDetector("test-resource", stubDetector{
		attrs: []attribute.KeyValue{attribute.String("vendor.attr", "detected")},
	})

	tests := []struct {
		name       string
		cfg        Config
		wantAttrs  map[string]string
		absentKeys []string
	}{
		{
			name: "resource carries service name, framework version and OTEL_RESOURCE_ATTRIBUTES",
			cfg:  Config{AppName: "app", Exporter: "test-resource"},
			wantAttrs: map[string]string{
				"service.name":           "app",
				"deployment.environment": "staging",
				"vendor.attr":            "detected",
			},
		},
		{
			name:       "detector does not run for a different exporter",
			cfg:        Config{AppName: "app", Exporter: "test-build-ok"},
			wantAttrs:  map[string]string{"service.name": "app"},
			absentKeys: []string{"vendor.attr"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			Register("test-build-ok", stubBuilder)

			res := buildResource(t.Context(), &tt.cfg, logging.NewMockLogger(logging.ERROR))

			got := map[string]string{}
			for _, kv := range res.Attributes() {
				got[string(kv.Key)] = kv.Value.AsString()
			}

			for k, v := range tt.wantAttrs {
				if got[k] != v {
					t.Errorf("resource attribute %q = %q, want %q", k, got[k], v)
				}
			}

			for _, k := range tt.absentKeys {
				if _, ok := got[k]; ok {
					t.Errorf("resource attribute %q should not be present", k)
				}
			}

			if _, ok := got["framework_version"]; !ok {
				t.Error("expected framework_version on the resource")
			}
		})
	}
}

func Test_Build_usesTheConfiguredSampler(t *testing.T) {
	Register("test-sampler", stubBuilder)

	cfg := Config{AppName: "app", Exporter: "test-sampler", Ratio: 0}

	shutdown, tp := Build(t.Context(), &cfg, logging.NewMockLogger(logging.ERROR))
	defer func() { _ = shutdown(t.Context()) }()

	_, span := tp.Tracer("test").Start(t.Context(), "span")
	defer span.End()

	if span.IsRecording() {
		t.Error("TRACER_RATIO=0 must not sample spans")
	}

	if !span.SpanContext().SpanID().IsValid() {
		t.Error("an unsampled span must still carry a valid SpanID for correlation IDs")
	}
}

func Test_Build_publishesTheResourceBeforeTheBuilderRuns(t *testing.T) {
	var seen *Config

	Register("test-resource-seen", func(ctx context.Context, cfg *Config, _ Logger) (sdktrace.SpanExporter, error) {
		seen = cfg
		return stubBuilder(ctx, cfg, nil)
	})

	cfg := Config{AppName: "app", Exporter: "test-resource-seen", Ratio: 1}

	shutdown, _ := Build(t.Context(), &cfg, logging.NewMockLogger(logging.ERROR))
	defer func() { _ = shutdown(t.Context()) }()

	if seen == nil || seen.Resource == nil {
		t.Fatal("expected Build to publish cfg.Resource before invoking the builder")
	}
}
