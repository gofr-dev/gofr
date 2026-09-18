//go:build !gofr_nootlp

package gofr

import (
	"context"
	"net"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"

	"gofr.dev/pkg/gofr/config"
	"gofr.dev/pkg/gofr/container"
	"gofr.dev/pkg/gofr/logging"
	"gofr.dev/pkg/gofr/testutil"
)

// The OTLP-transport cases of Test_initTracer, which exist only in a build
// without -tags gofr_nootlp. The zipkin and gofr cases stay in gofr_test.go:
// neither goes through OTLP, so both work in either build.
func Test_initTracer_otlpTransport(t *testing.T) {
	mockConfig := func(traceExporter, authKey string) config.Config {
		return config.NewMockConfig(map[string]string{
			"TRACE_EXPORTER":  traceExporter,
			"TRACER_URL":      "localhost:4317",
			"TRACER_AUTH_KEY": authKey,
		})
	}

	tests := []struct {
		desc               string
		config             config.Config
		expectedLogMessage string
	}{
		{"jaeger exporter", mockConfig("jaeger", ""), "Exporting traces to jaeger at localhost:4317"},
		{"jaeger exporter with auth", mockConfig("jaeger", "valid-token"), "Exporting traces to jaeger at localhost:4317"},
		{"otlp exporter", mockConfig("otlp", ""), "Exporting traces to otlp at localhost:4317"},
		{"otlp exporter with authKey", mockConfig("otlp", "valid-token"), "Exporting traces to otlp at localhost:4317"},
	}

	for i, tc := range tests {
		logMessage := testutil.StdoutOutputForFunc(func() {
			mockContainer, _ := container.NewMockContainer(t)

			a := App{Config: tc.config, container: mockContainer}
			a.initTracer()
		})
		assert.Contains(t, logMessage, tc.expectedLogMessage, "TEST[%d], Failed.\n%s", i, tc.desc)
	}
}

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
