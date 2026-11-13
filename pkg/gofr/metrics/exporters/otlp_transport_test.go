//go:build !gofr_nootlp

package exporters

import (
	"context"
	"strings"
	"testing"

	"gofr.dev/pkg/gofr/logging"
	"gofr.dev/pkg/gofr/testutil"
)

// Tests for the OTLP wire transports, which exist only in a build without
// -tags gofr_nootlp. Moved out of otlp_test.go alongside the code they cover.

func Test_buildOTLPExporter(t *testing.T) {
	tests := []struct {
		name string
		cfg  Config
	}{
		{"grpc default", Config{Endpoint: "localhost:4317", Protocol: "grpc", Insecure: true}},
		{"grpc with headers", Config{Endpoint: "localhost:4317", Protocol: "grpc", Insecure: true, Headers: map[string]string{"x": "y"}}},
		{"grpc secure with delta", Config{Endpoint: "otlp.nr-data.net:4317", Protocol: "grpc", Temporality: "delta"}},
		{"http host:port", Config{Endpoint: "localhost:4318", Protocol: "http", Insecure: true}},
		{"http full url", Config{Endpoint: "https://collector.example.com/v1/metrics", Protocol: "http"}},
		{"grpc https scheme ignores insecure override", Config{
			Endpoint: "https://collector.example.com:4317", Protocol: "grpc", Insecure: true,
		}},
		{"http https scheme ignores insecure override", Config{
			Endpoint: "https://collector.example.com/v1/metrics", Protocol: "http", Insecure: true,
		}},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			exp, err := buildOTLPExporter(context.Background(), &tc.cfg)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if exp == nil {
				t.Fatal("expected non-nil exporter")
			}

			_ = exp.Shutdown(context.Background())
		})
	}
}

func Test_buildOTLPReader_warnsOnUnrecognizedProtocolAndTemporality(t *testing.T) {
	tests := []struct {
		name    string
		cfg     Config
		wantLog string
	}{
		{"unrecognized protocol", Config{Endpoint: "localhost:4317", Protocol: "carrier-pigeon"}, "METRICS_PROTOCOL"},
		{"unrecognized temporality", Config{Endpoint: "localhost:4317", Temporality: "quantum"}, "METRICS_TEMPORALITY"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			out := testutil.StdoutOutputForFunc(func() {
				_, err := buildOTLPReader(context.Background(), &tc.cfg, logging.NewMockLogger(logging.WARN))
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
			})

			if !strings.Contains(out, tc.wantLog) {
				t.Errorf("expected warning to mention %q, got: %q", tc.wantLog, out)
			}
		})
	}
}
