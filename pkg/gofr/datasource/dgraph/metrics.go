package dgraph

import (
	"context"

	"github.com/prometheus/client_golang/prometheus"
)

type Metrics interface {
	NewHistogram(name, desc string, buckets ...float64)

	RecordHistogram(ctx context.Context, name string, value float64, labels ...string)
}

type PrometheusMetrics struct {
	histograms map[string]*prometheus.HistogramVec
}

// NewHistogram creates a new histogram metric with the given name, description, and optional bucket sizes.
func (p *PrometheusMetrics) NewHistogram(name, desc string, buckets ...float64) {
	histogram := prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    name,
			Help:    desc,
			Buckets: buckets,
		},
		[]string{}, // labels can be added here if needed
	)
	p.histograms[name] = histogram
	prometheus.MustRegister(histogram)
}

// RecordHistogram records a value to the specified histogram metric with optional labels.
func (p *PrometheusMetrics) RecordHistogram(_ context.Context, name string, value float64, labels ...string) {
	histogram, exists := p.histograms[name]
	if !exists {
		// Handle error: histogram not found
		return
	}

	histogram.WithLabelValues(labels...).Observe(value)
}
