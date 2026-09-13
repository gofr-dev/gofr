package exporters

import (
	"context"
	"errors"
	"testing"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
)

var errBuilder = errors.New("builder failed")

type stubDetector struct {
	attrs []attribute.KeyValue
}

func (d stubDetector) Detect(context.Context) (*resource.Resource, error) {
	return resource.NewSchemaless(d.attrs...), nil
}

func stubBuilder(context.Context, *Config, Logger) (sdktrace.SpanExporter, error) {
	return tracetest.NewNoopExporter(), nil
}

func failingBuilder(context.Context, *Config, Logger) (sdktrace.SpanExporter, error) {
	return nil, errBuilder
}

// Registrations are package-global and deliberately not undone: every test here
// uses a name of its own, so nothing an application could select is affected.
func Test_Register_and_lookup(t *testing.T) {
	Register("test-lookup", stubBuilder)

	tests := []struct {
		name      string
		lookup    string
		wantFound bool
	}{
		{name: "registered name is found", lookup: "test-lookup", wantFound: true},
		{name: "unregistered name is not found", lookup: "test-absent", wantFound: false},
		{name: "lookup is case sensitive, callers normalize", lookup: "TEST-LOOKUP", wantFound: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, found := lookup(tt.lookup)

			if found != tt.wantFound {
				t.Errorf("lookup(%q) found = %v, want %v", tt.lookup, found, tt.wantFound)
			}
		})
	}
}

func Test_Register_overridesExistingName(t *testing.T) {
	Register("test-override", failingBuilder)
	Register("test-override", stubBuilder)

	build, found := lookup("test-override")
	if !found {
		t.Fatal("expected the name to be registered")
	}

	if _, err := build(t.Context(), &Config{}, noopLogger{}); err != nil {
		t.Errorf("expected the last registration to win, got error: %v", err)
	}
}

func Test_RegisterResourceDetector_and_lookupDetector(t *testing.T) {
	RegisterResourceDetector("test-detector", stubDetector{})

	tests := []struct {
		name      string
		lookup    string
		wantFound bool
	}{
		{name: "registered detector is found", lookup: "test-detector", wantFound: true},
		{name: "unregistered detector is not found", lookup: "test-no-detector", wantFound: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, found := lookupDetector(tt.lookup)

			if found != tt.wantFound {
				t.Errorf("lookupDetector(%q) found = %v, want %v", tt.lookup, found, tt.wantFound)
			}
		})
	}
}

func Test_knownExternalExporters_pointsAtTheSubmodule(t *testing.T) {
	path, ok := knownExternalExporters[exporterGCP]
	if !ok {
		t.Fatalf("expected %q to be a known external exporter", exporterGCP)
	}

	if path != "gofr.dev/pkg/gofr/traces/exporters/gcp" {
		t.Errorf("unexpected import path hint: %q", path)
	}
}
