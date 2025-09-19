package exporters

import (
	"context"
	"os"
	"runtime"
	"time"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetrichttp"
	"go.opentelemetry.io/otel/metric"
	metricSdk "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/metric/metricdata"
	"go.opentelemetry.io/otel/sdk/resource"
	semconv "go.opentelemetry.io/otel/semconv/v1.17.0"

	"gofr.dev/pkg/gofr/version"
)

const (
	telemetryEndpoint = "localhost:8080"
	defaultAppName    = "gofr-app"
)

// SendFrameworkStartupTelemetry sends proper OTLP telemetry ONCE on startup.
func SendFrameworkStartupTelemetry(appName, appVersion string) {
	if os.Getenv("GOFR_TELEMETRY_DISABLED") == "true" {
		return
	}

	// Send in background to avoid blocking startup
	go func() {
		// Create OTLP HTTP exporter
		exporter, err := otlpmetrichttp.New(
			context.Background(),
			otlpmetrichttp.WithEndpoint(telemetryEndpoint),
			otlpmetrichttp.WithInsecure(),
			otlpmetrichttp.WithHeaders(map[string]string{
				"Content-Type": "application/x-protobuf", // Proper OTLP content type
			}),
		)
		if err != nil {
			return // Fail silently
		}

		// Create manual reader for one-time export
		reader := metricSdk.NewManualReader()

		// Create meter provider
		provider := metricSdk.NewMeterProvider(
			metricSdk.WithReader(reader),
			metricSdk.WithResource(createTelemetryResource(appName, appVersion)),
		)

		defer func() {
			if err = provider.Shutdown(context.Background()); err != nil {
				return
			}
		}()

		defer func() {
			if err = exporter.Shutdown(context.Background()); err != nil {
				return
			}
		}()

		// Get meter
		meter := provider.Meter("gofr-telemetry", metric.WithInstrumentationVersion(appVersion))

		// Create startup counter
		startupCounter, err := meter.Int64Counter(
			"gofr_framework_starts_total",
			metric.WithDescription("Total number of GoFr framework startups"),
		)
		if err != nil {
			return
		}

		if appName == "" {
			appName = defaultAppName
		}

		if appVersion == "" {
			appVersion = "unknown"
		}

		// Record startup event
		ctx := context.Background()
		startupCounter.Add(ctx, 1,
			metric.WithAttributes(
				attribute.String("app_name", appName),
				attribute.String("app_version", appVersion),
				attribute.String("startup_time", time.Now().UTC().Format(time.RFC3339)),
			),
		)

		var resourceMetrics metricdata.ResourceMetrics

		// Manually collect and export the metrics ONCE
		err = reader.Collect(ctx, &resourceMetrics)
		if err != nil {
			return
		}

		// Export the metrics
		if err := exporter.Export(ctx, &resourceMetrics); err != nil {
			return
		}
	}()
}

// createTelemetryResource creates resource with framework-specific attributes.
func createTelemetryResource(appName, appVersion string) *resource.Resource {
	return resource.NewWithAttributes(
		semconv.SchemaURL,
		semconv.ServiceNameKey.String(appName),
		semconv.ServiceVersionKey.String(appVersion),
		attribute.String("framework", "gofr"),
		attribute.String("framework_version", version.Framework),
		attribute.String("go_version", runtime.Version()),
		attribute.String("os", runtime.GOOS),
		attribute.String("arch", runtime.GOARCH),
	)
}
