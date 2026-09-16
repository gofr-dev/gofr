package gofr

import (
	"context"
	"errors"
	"net"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
	"go.opentelemetry.io/otel/trace/noop"

	"gofr.dev/pkg/gofr/config"
	"gofr.dev/pkg/gofr/container"
	"gofr.dev/pkg/gofr/logging"
	"gofr.dev/pkg/gofr/testutil"
)

func Test_resolveOtlpTransport(t *testing.T) {
	tests := []struct {
		name        string
		url         string
		insecure    bool
		insecureSet bool
		expected    otlpTransport
	}{
		{
			name:     "schemeless endpoint defaults to plaintext",
			url:      "collector:4317",
			insecure: true,
			expected: otlpTransport{useEndpointURL: false, insecure: true, plaintext: true},
		},
		{
			name:        "schemeless endpoint with TRACER_INSECURE=false uses TLS",
			url:         "collector:4317",
			insecure:    false,
			insecureSet: true,
			expected:    otlpTransport{useEndpointURL: false, insecure: false, plaintext: false},
		},
		{
			name:     "http scheme is plaintext",
			url:      "http://collector:4317",
			insecure: true,
			expected: otlpTransport{useEndpointURL: true, insecure: false, plaintext: true},
		},
		{
			name:     "https scheme uses TLS",
			url:      "https://collector:4317",
			insecure: true,
			expected: otlpTransport{useEndpointURL: true, insecure: false, plaintext: false},
		},
		{
			name:        "https scheme wins over TRACER_INSECURE=true",
			url:         "https://collector:4317",
			insecure:    true,
			insecureSet: true,
			expected:    otlpTransport{useEndpointURL: true, insecure: false, plaintext: false},
		},
		{
			name:        "http scheme wins over TRACER_INSECURE=false",
			url:         "http://collector:4317",
			insecure:    false,
			insecureSet: true,
			expected:    otlpTransport{useEndpointURL: true, insecure: false, plaintext: true},
		},
		{
			name:     "deprecated host:port path is plaintext by default",
			url:      "localhost:9411",
			insecure: true,
			expected: otlpTransport{useEndpointURL: false, insecure: true, plaintext: true},
		},
		{
			name:     "scheme match is case-insensitive",
			url:      "HTTPS://collector:4317",
			insecure: true,
			expected: otlpTransport{useEndpointURL: true, insecure: false, plaintext: false},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := resolveOtlpTransport(logging.NewMockLogger(logging.ERROR), tt.url, tt.insecure, tt.insecureSet)

			require.Equal(t, tt.expected, got)
		})
	}
}

// Test_resolveOtlpTransport_httpsIsNotDowngraded guards the invariant that makes
// the fix correct: an https:// endpoint resolves to WithEndpointURL alone. Pairing
// it with WithInsecure() would override the scheme, because the SDK applies
// options in slice order.
func Test_resolveOtlpTransport_httpsIsNotDowngraded(t *testing.T) {
	got := resolveOtlpTransport(logging.NewMockLogger(logging.ERROR), "https://otelcol.local:4317", true, false)

	require.True(t, got.useEndpointURL, "an https:// endpoint must be passed to WithEndpointURL")
	require.False(t, got.insecure, "WithInsecure must never be applied to an https:// endpoint")
	require.False(t, got.plaintext, "an https:// endpoint must not resolve to plaintext")
}

func Test_resolveOtlpTransport_warnsWhenInsecureIgnored(t *testing.T) {
	out := testutil.StdoutOutputForFunc(func() {
		resolveOtlpTransport(logging.NewMockLogger(logging.WARN), "https://collector:4317", true, true)
	})

	require.Contains(t, out, "TRACER_INSECURE")
}

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

func Test_buildOtlpExporter_warnsOnPlaintextCredentials(t *testing.T) {
	out := testutil.StdoutOutputForFunc(func() {
		exp, err := buildOtlpExporter(logging.NewMockLogger(logging.WARN), "otlp", "collector:4317", "", "",
			map[string]string{"Authorization": "Bearer token"}, true, false)
		require.NoError(t, err)
		require.NotNil(t, exp)

		_ = exp.Shutdown(t.Context())
	})

	require.Contains(t, out, "plaintext")
}

func Test_buildOtlpExporter(t *testing.T) {
	tests := []struct {
		name        string
		url         string
		host        string
		port        string
		insecure    bool
		insecureSet bool
	}{
		{name: "schemeless default", url: "collector:4317", insecure: true},
		{name: "schemeless with TLS", url: "collector:4317", insecure: false, insecureSet: true},
		{name: "http scheme", url: "http://collector:4317", insecure: true},
		{name: "https scheme", url: "https://collector:4317", insecure: true},
		{name: "deprecated host and port", host: "localhost", port: "9411", insecure: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			exp, err := buildOtlpExporter(logging.NewMockLogger(logging.ERROR), "otlp", tt.url, tt.host, tt.port,
				nil, tt.insecure, tt.insecureSet)

			require.NoError(t, err)
			require.NotNil(t, exp)

			_ = exp.Shutdown(t.Context())
		})
	}
}

// Test_buildOtlpExporter_wireTransport asserts the bytes the exporter actually
// puts on the socket, which is the only assertion that fails if the option block
// regresses: Test_buildOtlpExporter's err == nil && exp != nil holds for any
// option set, and resolveOtlpTransport is a pure function the wiring could stop
// consulting. Restoring the pre-fix line (WithInsecure + WithEndpoint(url)) makes
// both scheme-bearing subtests fail here — the raw URL is an invalid gRPC target,
// so no connection is ever opened.
func Test_buildOtlpExporter_wireTransport(t *testing.T) {
	const (
		h2cPreface   = "PRI * HTTP/2.0"
		tlsHandshake = 0x16
	)

	tests := []struct {
		name        string
		scheme      string
		insecure    bool
		insecureSet bool
		expectTLS   bool
	}{
		{name: "schemeless endpoint speaks h2c", scheme: "", insecure: true},
		{name: "http scheme speaks h2c", scheme: "http://", insecure: true},
		{name: "https scheme speaks TLS", scheme: "https://", insecure: true, expectTLS: true},
		{
			name: "schemeless with TRACER_INSECURE=false speaks TLS", scheme: "", insecure: false,
			insecureSet: true, expectTLS: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			first := make(chan []byte, 1)
			url := tt.scheme + listenForFirstBytes(t, first)

			exp, err := buildOtlpExporter(logging.NewMockLogger(logging.ERROR), "otlp", url, "", "",
				nil, tt.insecure, tt.insecureSet)
			require.NoError(t, err)

			t.Cleanup(func() { _ = exp.Shutdown(context.Background()) })

			// The export itself always errors — nothing on the far end speaks OTLP.
			// What is under test is whether a connection happened at all, and in
			// which protocol. A short deadline keeps the TLS case off the SDK default.
			ctx, cancel := context.WithTimeout(t.Context(), time.Second)
			defer cancel()

			_ = exp.ExportSpans(ctx, tracetest.SpanStubs{{Name: "wire-probe"}}.Snapshots())

			select {
			case got := <-first:
				require.NotEmpty(t, got, "connection opened but no bytes were sent")

				if tt.expectTLS {
					require.EqualValues(t, tlsHandshake, got[0], "expected a TLS ClientHello, got %q", got)
				} else {
					require.Contains(t, string(got), h2cPreface, "expected the h2c preface")
				}
			case <-time.After(2 * time.Second):
				t.Fatalf("no connection was made to %s: the endpoint is not a usable gRPC target", url)
			}
		})
	}
}

// listenForFirstBytes starts a throwaway listener and hands the first bytes of
// the first connection to first. It returns the host:port to dial.
func listenForFirstBytes(t *testing.T, first chan<- []byte) string {
	t.Helper()

	var lc net.ListenConfig

	ln, err := lc.Listen(t.Context(), "tcp", "127.0.0.1:0")
	require.NoError(t, err)

	t.Cleanup(func() { _ = ln.Close() })

	go func() {
		conn, err := ln.Accept()
		if err != nil {
			return
		}

		defer conn.Close()

		_ = conn.SetReadDeadline(time.Now().Add(2 * time.Second))

		buf := make([]byte, 24)
		n, _ := conn.Read(buf)

		first <- buf[:n]
	}()

	return ln.Addr().String()
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

func Test_redactURL(t *testing.T) {
	tests := []struct {
		name     string
		raw      string
		expected string
	}{
		{name: "schemeless host:port is unchanged", raw: "collector:4317", expected: "collector:4317"},
		{name: "plain https URL is unchanged", raw: "https://collector:4317", expected: "https://collector:4317"},
		{name: "path is kept", raw: "http://localhost:2005/api/v2/spans", expected: "http://localhost:2005/api/v2/spans"},
		{name: "empty is unchanged", raw: "", expected: ""},
		{name: "userinfo with password", raw: "https://user:s3cret@collector:4317", expected: "https://REDACTED@collector:4317"},
		{name: "userinfo token only", raw: "https://t0ken@collector:4317", expected: "https://REDACTED@collector:4317"},
		{name: "query credentials", raw: "https://zipkin/api/v2/spans?api-key=s3cret", expected: "https://zipkin/api/v2/spans?REDACTED"},
		{name: "fragment is dropped", raw: "https://collector:4317#s3cret", expected: "https://collector:4317"},
		{name: "schemeless userinfo", raw: "user:s3cret@collector:4317", expected: "REDACTED@collector:4317"},
		{name: "unparsable with userinfo", raw: "https://user:s3cret@collector:43%17", expected: "REDACTED@collector:43%17"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.Equal(t, tt.expected, redactURL(tt.raw))
		})
	}
}

// Test_initTracer_doesNotLogCredentials drives the real startup path for every
// exporter that logs its endpoint, and asserts the credentials in TRACER_URL and
// the raw TRACER_INSECURE value never reach the log.
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
		{name: "gofr userinfo", exporter: "gofr", url: "https://user:" + secret + "@tracer.example.com/api/spans",
			expected: "Exporting traces to GoFr at https://REDACTED@tracer.example.com/api/spans"},
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
