package exporters

import (
	"context"
	"testing"

	metricSdk "go.opentelemetry.io/otel/sdk/metric"

	"gofr.dev/pkg/gofr/logging"
)

func TestRegister(t *testing.T) {
	called := 0

	Register("test-register", func(_ context.Context, _ *Config, _ Logger) (metricSdk.Reader, error) {
		called++
		return metricSdk.NewManualReader(), nil
	})

	b, ok := lookup("test-register")
	if !ok {
		t.Fatal("expected registered builder to be found via lookup")
	}

	if _, err := b(context.Background(), &Config{}, logging.NewMockLogger(logging.INFO)); err != nil {
		t.Fatalf("builder returned error: %v", err)
	}

	if called != 1 {
		t.Errorf("expected builder to be invoked once, got %d", called)
	}

	// Registering the same name overrides the previous builder.
	Register("test-register", func(_ context.Context, _ *Config, _ Logger) (metricSdk.Reader, error) {
		return metricSdk.NewManualReader(), nil
	})

	if _, ok := lookup("test-register"); !ok {
		t.Error("expected overridden builder to remain registered")
	}

	if _, ok := lookup("does-not-exist"); ok {
		t.Error("expected lookup of unknown name to fail")
	}
}
