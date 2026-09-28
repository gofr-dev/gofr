package exporters

import (
	"strings"
	"testing"

	"gofr.dev/pkg/gofr/logging"
	"gofr.dev/pkg/gofr/testutil"
)

func Test_zipkinEndpoint(t *testing.T) {
	tests := []struct {
		name     string
		cfg      Config
		expected string
	}{
		{
			name:     "endpoint wins",
			cfg:      Config{Endpoint: "http://zipkin:9411/api/v2/spans", Host: "h", Port: "9411"},
			expected: "http://zipkin:9411/api/v2/spans",
		},
		{
			name:     "falls back to the deprecated host and port",
			cfg:      Config{Host: "localhost", Port: "9411"},
			expected: "http://localhost:9411/api/v2/spans",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := zipkinEndpoint(&tt.cfg); got != tt.expected {
				t.Errorf("zipkinEndpoint() = %q, want %q", got, tt.expected)
			}
		})
	}
}

func Test_buildZipkinExporter(t *testing.T) {
	tests := []struct {
		name string
		cfg  Config
	}{
		{name: "url", cfg: Config{Endpoint: "http://zipkin:9411/api/v2/spans"}},
		{name: "deprecated host and port", cfg: Config{Host: "localhost", Port: "9411"}},
		{name: "headers", cfg: Config{Endpoint: "http://zipkin:9411/api/v2/spans", Headers: map[string]string{"k": "v"}}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			exp, err := buildZipkinExporter(t.Context(), &tt.cfg, logging.NewMockLogger(logging.ERROR))
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

func Test_buildZipkinExporter_warnsThatItIsDeprecated(t *testing.T) {
	out := testutil.StdoutOutputForFunc(func() {
		cfg := Config{Endpoint: "http://zipkin:9411/api/v2/spans"}

		exp, err := buildZipkinExporter(t.Context(), &cfg, logging.NewMockLogger(logging.WARN))
		if err != nil {
			t.Errorf("unexpected error: %v", err)
			return
		}

		_ = exp.Shutdown(t.Context())
	})

	if !strings.Contains(out, "deprecated") {
		t.Errorf("expected a deprecation warning, got: %q", out)
	}
}
