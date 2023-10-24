package gofr

import "gofr.dev/pkg/gofr/metrics"

// NewCounter registers new custom counter metric
func (k *Gofr) NewCounter(name, help string, labels ...string) error {
	if k.Metric == nil {
		k.Metric = metrics.NewMetric()
	}

	return metrics.NewCounter(k.Metric, name, help, labels...)
}

// NewHistogram registers new custom histogram metric
func (k *Gofr) NewHistogram(name, help string, buckets []float64, labels ...string) error {
	if k.Metric == nil {
		k.Metric = metrics.NewMetric()
	}

	return metrics.NewHistogram(k.Metric, name, help, buckets, labels...)
}

// NewGauge registers new custom gauge metric
func (k *Gofr) NewGauge(name, help string, labels ...string) error {
	if k.Metric == nil {
		k.Metric = metrics.NewMetric()
	}

	return metrics.NewGauge(k.Metric, name, help, labels...)
}

// NewSummary registers new custom summary metric
func (k *Gofr) NewSummary(name, help string, labels ...string) error {
	if k.Metric == nil {
		k.Metric = metrics.NewMetric()
	}

	return metrics.NewSummary(k.Metric, name, help, labels...)
}
