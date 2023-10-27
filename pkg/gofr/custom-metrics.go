package gofr

import "gofr.dev/pkg/gofr/metrics"

// NewCounter registers new custom counter metric
func (g *Gofr) NewCounter(name, help string, labels ...string) error {
	if g.Metric == nil {
		g.Metric = metrics.NewMetric()
	}

	return metrics.NewCounter(g.Metric, name, help, labels...)
}

// NewHistogram registers new custom histogram metric
func (g *Gofr) NewHistogram(name, help string, buckets []float64, labels ...string) error {
	if g.Metric == nil {
		g.Metric = metrics.NewMetric()
	}

	return metrics.NewHistogram(g.Metric, name, help, buckets, labels...)
}

// NewGauge registers new custom gauge metric
func (g *Gofr) NewGauge(name, help string, labels ...string) error {
	if g.Metric == nil {
		g.Metric = metrics.NewMetric()
	}

	return metrics.NewGauge(g.Metric, name, help, labels...)
}

// NewSummary registers new custom summary metric
func (g *Gofr) NewSummary(name, help string, labels ...string) error {
	if g.Metric == nil {
		g.Metric = metrics.NewMetric()
	}

	return metrics.NewSummary(g.Metric, name, help, labels...)
}
