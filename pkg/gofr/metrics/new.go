package metrics

import (
	"gofr.dev/pkg/errors"
)

var (
	errInvalidType   = errors.Error("unable to create new metric, invalid type of Metric")
	errInvalidMetric = errors.Error("got nil Metric, on creating new custom metric")
)

// NewMetric factory function for custom metric
func NewMetric() Metric {
	return newPromVec()
}

// NewCounter adds new custom counter metric
func NewCounter(m Metric, name, help string, labels ...string) error {
	if m == nil {
		return errInvalidMetric
	}

	if p, ok := m.(*promVec); ok {
		return p.registerCounter(name, help, labels...)
	}

	return errInvalidType
}

// NewHistogram adds new custom Histogram metric
func NewHistogram(m Metric, name, help string, buckets []float64, labels ...string) error {
	if m == nil {
		return errInvalidMetric
	}

	if p, ok := m.(*promVec); ok {
		return p.registerHistogram(name, help, buckets, labels...)
	}

	return errInvalidType
}

// NewGauge adds new custom Gauge metric
func NewGauge(m Metric, name, help string, labels ...string) error {
	if m == nil {
		return errInvalidMetric
	}

	if p, ok := m.(*promVec); ok {
		return p.registerGauge(name, help, labels...)
	}

	return errInvalidType
}

// NewSummary add new custom Summary metric
func NewSummary(m Metric, name, help string, labels ...string) error {
	if m == nil {
		return errInvalidMetric
	}

	if p, ok := m.(*promVec); ok {
		return p.registerSummary(name, help, labels...)
	}

	return errInvalidType
}
