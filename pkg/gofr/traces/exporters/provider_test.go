package exporters

import (
	"context"
	"strings"
	"testing"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"

	"gofr.dev/pkg/gofr/logging"
	"gofr.dev/pkg/gofr/testutil"
	"gofr.dev/pkg/gofr/version"
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
		env        map[string]string
		wantAttrs  map[string]string
		absentKeys []string
		wantLog    string
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
		// service.name is the one key GoFr also supplies, and the case above cannot
		// see the precedence at all: deployment.environment collides with nothing.
		// These rows pin the precedence itself, that the override is announced, and
		// that framework_version is *not* reachable from the environment — so a
		// reshuffle of the resource.Option slice fails here rather than in
		// production. Keep them identical to metrics/exporters.Test_buildResource:
		// the two packages must resolve the same name or the trace↔metric join
		// breaks.
		{
			name:      "OTEL_SERVICE_NAME wins over APP_NAME",
			cfg:       Config{AppName: "app", Exporter: "test-build-ok"},
			env:       map[string]string{"OTEL_SERVICE_NAME": "from-env"},
			wantAttrs: map[string]string{"service.name": "from-env"},
			wantLog:   `service.name="from-env" from the environment overrides APP_NAME ("app")`,
		},
		{
			name: "service.name in OTEL_RESOURCE_ATTRIBUTES wins, its siblings survive",
			cfg:  Config{AppName: "app", Exporter: "test-build-ok"},
			env: map[string]string{
				"OTEL_RESOURCE_ATTRIBUTES": "service.name=from-env,deployment.environment=prod",
			},
			wantAttrs: map[string]string{"service.name": "from-env", "deployment.environment": "prod"},
			wantLog:   `service.name="from-env" from the environment overrides APP_NAME ("app")`,
		},
		{
			name: "OTEL_SERVICE_NAME outranks OTEL_RESOURCE_ATTRIBUTES",
			cfg:  Config{AppName: "app", Exporter: "test-build-ok"},
			env: map[string]string{
				"OTEL_SERVICE_NAME":        "from-var",
				"OTEL_RESOURCE_ATTRIBUTES": "service.name=from-attrs",
			},
			wantAttrs: map[string]string{"service.name": "from-var"},
			wantLog:   `service.name="from-var" from the environment overrides APP_NAME ("app")`,
		},
		{
			name:      "no log when the environment agrees with APP_NAME",
			cfg:       Config{AppName: "app", Exporter: "test-build-ok"},
			env:       map[string]string{"OTEL_SERVICE_NAME": "app"},
			wantAttrs: map[string]string{"service.name": "app"},
		},
		// OTEL_RESOURCE_ATTRIBUTES="service.name=" parses to a valid attribute with
		// an empty value, so an unguarded "env wins" ships a nameless service.
		{
			name:      "empty service.name falls back to APP_NAME",
			cfg:       Config{AppName: "app", Exporter: "test-build-ok"},
			env:       map[string]string{"OTEL_RESOURCE_ATTRIBUTES": "service.name="},
			wantAttrs: map[string]string{"service.name": "app"},
		},
		{
			name:      "framework_version is not overridable from the environment",
			cfg:       Config{AppName: "app", Exporter: "test-build-ok"},
			env:       map[string]string{"OTEL_RESOURCE_ATTRIBUTES": "framework_version=hacked"},
			wantAttrs: map[string]string{"framework_version": version.Framework},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			Register("test-build-ok", stubBuilder)

			for k, v := range tt.env {
				t.Setenv(k, v)
			}

			var res *resource.Resource

			// Infof goes to stdout: the framework logger reserves stderr for ERROR
			// and above.
			out := testutil.StdoutOutputForFunc(func() {
				res = buildResource(t.Context(), &tt.cfg, logging.NewMockLogger(logging.INFO))
			})

			assertServiceNameLog(t, out, tt.wantLog)
			assertResourceAttrs(t, res, tt.wantAttrs, tt.absentKeys)
		})
	}
}

func assertServiceNameLog(t *testing.T, out, wantLog string) {
	t.Helper()

	if wantLog != "" && !strings.Contains(out, wantLog) {
		t.Errorf("expected log to mention %q, got: %q", wantLog, out)
	}

	if wantLog == "" && strings.Contains(out, "overrides APP_NAME") {
		t.Errorf("unexpected service.name override log: %q", out)
	}
}

func assertResourceAttrs(t *testing.T, res *resource.Resource, wantAttrs map[string]string, absentKeys []string) {
	t.Helper()

	got := map[string]string{}
	for _, kv := range res.Attributes() {
		got[string(kv.Key)] = kv.Value.AsString()
	}

	for k, v := range wantAttrs {
		if got[k] != v {
			t.Errorf("resource attribute %q = %q, want %q", k, got[k], v)
		}
	}

	for _, k := range absentKeys {
		if _, ok := got[k]; ok {
			t.Errorf("resource attribute %q should not be present", k)
		}
	}

	if _, ok := got["framework_version"]; !ok {
		t.Error("expected framework_version on the resource")
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

// Build is exported, so a caller outside the framework can reach it without a
// logger. noopLogger is documented as the fallback for exactly that caller; this
// pins that it is actually substituted rather than dereferenced.
func Test_Build_substitutesNoopLoggerForANilLogger(t *testing.T) {
	Register("test-nil-logger", stubBuilder)

	tests := []struct {
		name     string
		exporter string
		env      map[string]string
	}{
		{name: "tracing disabled", exporter: ""},
		{name: "unknown exporter takes the degrade path", exporter: "no-such-exporter"},
		{name: "builder runs", exporter: "test-nil-logger"},
		// The service.name override logs, so the nil path must survive a call that
		// actually reaches the logger rather than only ones that skip it.
		{
			name:     "environment overrides service.name",
			exporter: "test-nil-logger",
			env:      map[string]string{"OTEL_SERVICE_NAME": "from-env"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			for k, v := range tt.env {
				t.Setenv(k, v)
			}

			cfg := Config{AppName: "app", Exporter: tt.exporter, Ratio: 1}

			shutdown, tp := Build(t.Context(), &cfg, nil)
			if shutdown == nil || tp == nil {
				t.Fatal("Build must never return a nil ShutdownFunc or provider")
			}

			_ = shutdown(t.Context())
		})
	}
}
