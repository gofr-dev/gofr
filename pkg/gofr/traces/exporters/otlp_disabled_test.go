//go:build gofr_nootlp

package exporters

import (
	"context"
	"errors"
	"strings"
	"testing"
)

// recordingLogger keeps the Errorf lines so a test can assert what an operator would actually read.
type recordingLogger struct {
	errors []string
}

func (*recordingLogger) Debug(...any)         {}
func (*recordingLogger) Infof(string, ...any) {}
func (*recordingLogger) Warnf(string, ...any) {}
func (l *recordingLogger) Errorf(format string, args ...any) {
	l.errors = append(l.errors, format)

	for _, a := range args {
		l.errors = append(l.errors, toText(a))
	}
}

func toText(v any) string {
	if err, ok := v.(error); ok {
		return err.Error()
	}

	s, _ := v.(string)

	return s
}

// Test_OTLPTraceExporterOmitted_StillRegistersItsNames pins the decision that makes this tag
// debuggable: the names stay registered.
//
// If they were simply absent, Build would report TRACE_EXPORTER=otlp as an unrecognized name, which
// reads like a typo and sends the operator to check their spelling. Registering a builder that fails
// with the tag in the message says the true thing instead -- the name is right, this binary does not
// carry it.
func Test_OTLPTraceExporterOmitted_StillRegistersItsNames(t *testing.T) {
	for _, name := range []string{exporterOTLP, exporterJaeger} {
		t.Run(name, func(t *testing.T) {
			builder, ok := lookup(name)
			if !ok {
				t.Fatalf("%q must stay registered under gofr_nootlp so the failure names the tag, not a typo", name)
			}

			logger := &recordingLogger{}

			exporter, err := builder(context.Background(), &Config{Exporter: name}, logger)

			if exporter != nil {
				t.Error("an omitted exporter must not return a SpanExporter")
			}

			if !errors.Is(err, errOTLPTracesOmitted) {
				t.Errorf("err = %v, want errOTLPTracesOmitted", err)
			}

			joined := strings.Join(logger.errors, " ")
			for _, want := range []string{"gofr_nootlp", "rebuild without the tag", name} {
				if !strings.Contains(joined, want) {
					t.Errorf("the logged error must mention %q; got %q", want, joined)
				}
			}
		})
	}
}

// Test_OTLPTraceExporterOmitted_BuildStillReturnsAProvider pins that configuring an omitted exporter
// degrades rather than crashes. Build's nil-exporter path installs the NeverSample provider, so
// trace and span IDs stay valid and X-Correlation-ID keeps working -- the service starts, logs the
// reason, and exports nothing.
func Test_OTLPTraceExporterOmitted_BuildStillReturnsAProvider(t *testing.T) {
	shutdown, tp := Build(t.Context(), &Config{AppName: "app", Exporter: exporterOTLP, Ratio: 1}, &recordingLogger{})

	defer func() { _ = shutdown(t.Context()) }()

	if tp == nil {
		t.Fatal("Build must return a usable TracerProvider even when the configured exporter is omitted")
	}

	_, span := tp.Tracer("test").Start(t.Context(), "span")
	defer span.End()

	if !span.SpanContext().TraceID().IsValid() {
		t.Error("spans must still carry a valid TraceID so correlation IDs keep working")
	}
}
