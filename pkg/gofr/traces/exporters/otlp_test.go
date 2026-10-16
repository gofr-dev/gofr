//go:build !gofr_nootlp

package exporters

import (
	"context"
	"net"
	"strings"
	"testing"
	"time"

	"go.opentelemetry.io/otel/sdk/trace/tracetest"

	"gofr.dev/pkg/gofr/logging"
	"gofr.dev/pkg/gofr/testutil"
)

func Test_resolveOtlpTransport(t *testing.T) {
	tests := []struct {
		name     string
		url      string
		cfg      Config
		expected otlpTransport
	}{
		{
			name:     "schemeless endpoint defaults to plaintext",
			url:      "collector:4317",
			cfg:      Config{Insecure: true},
			expected: otlpTransport{useEndpointURL: false, insecure: true, plaintext: true},
		},
		{
			name:     "schemeless endpoint with TRACER_INSECURE=false uses TLS",
			url:      "collector:4317",
			cfg:      Config{Insecure: false, InsecureSet: true},
			expected: otlpTransport{useEndpointURL: false, insecure: false, plaintext: false},
		},
		{
			name:     "http scheme is plaintext",
			url:      "http://collector:4317",
			cfg:      Config{Insecure: true},
			expected: otlpTransport{useEndpointURL: true, insecure: false, plaintext: true},
		},
		{
			name:     "https scheme uses TLS",
			url:      "https://collector:4317",
			cfg:      Config{Insecure: true},
			expected: otlpTransport{useEndpointURL: true, insecure: false, plaintext: false},
		},
		{
			name:     "https scheme wins over TRACER_INSECURE=true",
			url:      "https://collector:4317",
			cfg:      Config{Insecure: true, InsecureSet: true},
			expected: otlpTransport{useEndpointURL: true, insecure: false, plaintext: false},
		},
		{
			name:     "http scheme wins over TRACER_INSECURE=false",
			url:      "http://collector:4317",
			cfg:      Config{Insecure: false, InsecureSet: true},
			expected: otlpTransport{useEndpointURL: true, insecure: false, plaintext: true},
		},
		{
			name:     "deprecated host:port path is plaintext by default",
			url:      "localhost:9411",
			cfg:      Config{Insecure: true},
			expected: otlpTransport{useEndpointURL: false, insecure: true, plaintext: true},
		},
		{
			name:     "scheme match is case-insensitive",
			url:      "HTTPS://collector:4317",
			cfg:      Config{Insecure: true},
			expected: otlpTransport{useEndpointURL: true, insecure: false, plaintext: false},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := resolveOtlpTransport(&tt.cfg, tt.url, logging.NewMockLogger(logging.ERROR))

			if got != tt.expected {
				t.Errorf("resolveOtlpTransport(%q) = %+v, want %+v", tt.url, got, tt.expected)
			}
		})
	}
}

// Test_resolveOtlpTransport_httpsIsNotDowngraded guards the invariant that makes
// the fix for gofr-dev/gofr#4204 correct: an https:// endpoint resolves to
// WithEndpointURL alone. Pairing it with WithInsecure() would override the
// scheme, because the SDK applies options in slice order.
func Test_resolveOtlpTransport_httpsIsNotDowngraded(t *testing.T) {
	got := resolveOtlpTransport(&Config{Insecure: true}, "https://otelcol.local:4317", logging.NewMockLogger(logging.ERROR))

	if !got.useEndpointURL {
		t.Error("an https:// endpoint must be passed to WithEndpointURL")
	}

	if got.insecure || got.plaintext {
		t.Error("WithInsecure must never be applied to an https:// endpoint")
	}
}

func Test_resolveOtlpTransport_warnsWhenInsecureIgnored(t *testing.T) {
	out := testutil.StdoutOutputForFunc(func() {
		resolveOtlpTransport(&Config{Insecure: true, InsecureSet: true}, "https://collector:4317",
			logging.NewMockLogger(logging.WARN))
	})

	if !strings.Contains(out, "TRACER_INSECURE") {
		t.Errorf("expected a warning naming TRACER_INSECURE, got: %q", out)
	}
}

func Test_buildOtlpExporter_warnsOnPlaintextCredentials(t *testing.T) {
	out := testutil.StdoutOutputForFunc(func() {
		cfg := Config{
			Exporter: "otlp",
			Endpoint: "collector:4317",
			Insecure: true,
			Headers:  map[string]string{"Authorization": "Bearer token"},
		}

		exp, err := buildOtlpExporter(t.Context(), exporterOTLP, &cfg, logging.NewMockLogger(logging.WARN))
		if err != nil {
			t.Errorf("unexpected error: %v", err)
			return
		}

		_ = exp.Shutdown(t.Context())
	})

	if !strings.Contains(out, "plaintext") {
		t.Errorf("expected a plaintext-credentials warning, got: %q", out)
	}
}

func Test_buildOtlpExporter(t *testing.T) {
	tests := []struct {
		name string
		cfg  Config
	}{
		{name: "schemeless default", cfg: Config{Endpoint: "collector:4317", Insecure: true}},
		{name: "schemeless with TLS", cfg: Config{Endpoint: "collector:4317", InsecureSet: true}},
		{name: "http scheme", cfg: Config{Endpoint: "http://collector:4317", Insecure: true}},
		{name: "https scheme", cfg: Config{Endpoint: "https://collector:4317", Insecure: true}},
		{name: "deprecated host and port", cfg: Config{Host: "localhost", Port: "9411", Insecure: true}},
		{name: "headers", cfg: Config{Endpoint: "collector:4317", Headers: map[string]string{"X-Api-Key": "k"}}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			exp, err := buildOtlpExporter(t.Context(), exporterOTLP, &tt.cfg, logging.NewMockLogger(logging.ERROR))
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if exp == nil {
				t.Fatal("expected a non-nil exporter")
			}

			_ = exp.Shutdown(t.Context())
		})
	}
}

func Test_otlpEndpoint_defaultsToTheDeprecatedHostAndPort(t *testing.T) {
	tests := []struct {
		name     string
		cfg      Config
		expected string
	}{
		{name: "endpoint wins", cfg: Config{Endpoint: "collector:4317", Host: "h", Port: "9411"}, expected: "collector:4317"},
		{name: "falls back to host:port", cfg: Config{Host: "localhost", Port: "9411"}, expected: "localhost:9411"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := otlpEndpoint(&tt.cfg); got != tt.expected {
				t.Errorf("otlpEndpoint() = %q, want %q", got, tt.expected)
			}
		})
	}
}

// Test_buildOtlpExporter_wireTransport asserts the bytes the exporter actually
// puts on the socket, which is the only assertion that fails if the option block
// regresses: Test_buildOtlpExporter's err == nil && exp != nil holds for any
// option set, and resolveOtlpTransport is a pure function the wiring could stop
// consulting. Restoring the pre-fix line (WithInsecure + WithEndpoint(endpoint))
// makes both scheme-bearing subtests fail here — the raw URL is an invalid gRPC
// target, so no connection is ever opened.
func Test_buildOtlpExporter_wireTransport(t *testing.T) {
	const (
		h2cPreface   = "PRI * HTTP/2.0"
		tlsHandshake = 0x16
	)

	tests := []struct {
		name      string
		scheme    string
		cfg       Config
		expectTLS bool
	}{
		{name: "schemeless endpoint speaks h2c", cfg: Config{Insecure: true}},
		{name: "http scheme speaks h2c", scheme: schemeHTTP, cfg: Config{Insecure: true}},
		{name: "https scheme speaks TLS", scheme: schemeHTTPS, cfg: Config{Insecure: true}, expectTLS: true},
		{name: "schemeless with TRACER_INSECURE=false speaks TLS", cfg: Config{InsecureSet: true}, expectTLS: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			first := make(chan []byte, 1)

			cfg := tt.cfg
			cfg.Exporter = exporterOTLP
			cfg.Endpoint = tt.scheme + listenForFirstBytes(t, first)

			exp, err := buildOtlpExporter(t.Context(), exporterOTLP, &cfg, logging.NewMockLogger(logging.ERROR))
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			t.Cleanup(func() { _ = exp.Shutdown(context.Background()) })

			// The export itself always errors — nothing on the far end speaks OTLP.
			// What is under test is whether a connection happened at all, and in
			// which protocol. A short deadline keeps the TLS case off the SDK default.
			ctx, cancel := context.WithTimeout(t.Context(), time.Second)
			defer cancel()

			_ = exp.ExportSpans(ctx, tracetest.SpanStubs{{Name: "wire-probe"}}.Snapshots())

			select {
			case got := <-first:
				assertWireProtocol(t, got, tt.expectTLS, h2cPreface, tlsHandshake)
			case <-time.After(2 * time.Second):
				t.Fatalf("no connection was made to %s: the endpoint is not a usable gRPC target", cfg.Endpoint)
			}
		})
	}
}

func assertWireProtocol(t *testing.T, got []byte, expectTLS bool, h2cPreface string, tlsHandshake byte) {
	t.Helper()

	if len(got) == 0 {
		t.Fatal("connection opened but no bytes were sent")
	}

	if expectTLS {
		if got[0] != tlsHandshake {
			t.Errorf("expected a TLS ClientHello, got %q", got)
		}

		return
	}

	if !strings.Contains(string(got), h2cPreface) {
		t.Errorf("expected the h2c preface, got %q", got)
	}
}

// listenForFirstBytes starts a throwaway listener and hands the first bytes of
// the first connection to first. It returns the host:port to dial.
func listenForFirstBytes(t *testing.T, first chan<- []byte) string {
	t.Helper()

	var lc net.ListenConfig

	ln, err := lc.Listen(t.Context(), "tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}

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
