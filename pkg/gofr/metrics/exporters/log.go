package exporters

import (
	"context"
	"encoding/json"
	"strconv"

	"github.com/prometheus/client_golang/prometheus"
	dto "github.com/prometheus/client_model/go"
	otelprom "go.opentelemetry.io/otel/exporters/prometheus"
	"go.opentelemetry.io/otel/metric"
	sdk "go.opentelemetry.io/otel/sdk/metric"
)

type Logger interface {
	Infof(format string, args ...any)
}

// Log returns a meter provider that exports metrics to logs at the end of execution (when flush is called).
func Log(appName, appVersion string, logger Logger) (meter metric.Meter, flush func(context.Context) error) {
	registry := prometheus.NewRegistry()

	// Create OTel Prometheus exporter using the custom registry
	exporter, err := otelprom.New(otelprom.WithRegisterer(registry))
	if err != nil {
		return nil, func(context.Context) error { return nil }
	}

	// Create MeterProvider with the exporter
	provider := sdk.NewMeterProvider(sdk.WithReader(exporter))
	meter = provider.Meter(appName, metric.WithInstrumentationVersion(appVersion))

	flush = func(_ context.Context) error {
		metricFamilies, err := registry.Gather()
		if err != nil {
			return err
		}

		metrics := make(map[string]any)

		for _, mf := range metricFamilies {
			metrics[mf.GetName()] = convertMetricFamilyToMap(mf)
		}

		if len(metrics) == 0 {
			return nil
		}

		b, _ := json.Marshal(metrics)
		logger.Infof("[GOFR_METRICS] %s", string(b))

		return nil
	}

	return meter, flush
}

func convertMetricFamilyToMap(mf *dto.MetricFamily) []any {
	samples := make([]any, 0)

	for _, m := range mf.GetMetric() {
		sample := make(map[string]any)
		labels := make(map[string]string)

		for _, lp := range m.GetLabel() {
			labels[lp.GetName()] = lp.GetValue()
		}

		if len(labels) > 0 {
			sample["labels"] = labels
		}

		switch {
		case m.Gauge != nil:
			sample["value"] = m.GetGauge().GetValue()
		case m.Counter != nil:
			sample["value"] = m.GetCounter().GetValue()
		case m.Untyped != nil:
			sample["value"] = m.GetUntyped().GetValue()
		case m.Histogram != nil:
			h := m.GetHistogram()
			sample["count"] = h.GetSampleCount()
			sample["sum"] = h.GetSampleSum()

			buckets := make(map[string]uint64)
			for _, b := range h.GetBucket() {
				buckets[strconv.FormatFloat(b.GetUpperBound(), 'f', -1, 64)] = b.GetCumulativeCount()
			}

			sample["buckets"] = buckets
		}

		samples = append(samples, sample)
	}

	return samples
}
