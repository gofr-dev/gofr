package exporters

import (
	"context"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/push"
	otelprom "go.opentelemetry.io/otel/exporters/prometheus"
	"go.opentelemetry.io/otel/metric"
	sdk "go.opentelemetry.io/otel/sdk/metric"
)

// PrometheusPush returns a meter provider that pushes metrics to a Prometheus Pushgateway.
// It returns the Meter provider and a flush function that should be called on shutdown.
func PrometheusPush(appName, appVersion, pushgatewayURL, jobName string, interval int) (meter metric.Meter,
	flush func(context.Context) error) {
	// Create a new Prometheus registry
	registry := prometheus.NewRegistry()

	// Create OTel Prometheus exporter using the custom registry
	exporter, err := otelprom.New(otelprom.WithRegisterer(registry))
	if err != nil {
		// If exporter creation fails, return a no-op meter and a no-op flush
		return nil, func(context.Context) error { return nil }
	}

	// Create MeterProvider with the exporter
	provider := sdk.NewMeterProvider(sdk.WithReader(exporter))
	meter = provider.Meter(appName, metric.WithInstrumentationVersion(appVersion))

	// Create a pusher that pushes to the configured gateway
	pusher := push.New(pushgatewayURL, jobName).Gatherer(registry)

	// Create the flush function
	flush = func(_ context.Context) error {
		return pusher.Push()
	}

	// Start a background goroutine to push metrics periodically if interval > 0
	if interval > 0 {
		go func() {
			ticker := time.NewTicker(time.Duration(interval) * time.Second)
			defer ticker.Stop()

			for range ticker.C {
				_ = pusher.Push()
			}
		}()
	}

	return meter, flush
}
