package metrics

import (
	"context"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
)

// Error can also be returned from all the methods, but it is decided not to do so such that to keep the usage clean -
// as any errors are already being logged from here. Otherwise, user would need to check the error everytime.

type Manager interface {
	// NewCounter registers a new counter metrics. It can not be reduced.
	NewCounter(name, desc string)
	// NewUpDownCounter registers a new UpDown Counter metrics which can be either be increased or decreased by value.
	NewUpDownCounter(name, desc string)
	// NewHistogram registers a new histogram metrics with different buckets.
	NewHistogram(name, desc string, buckets ...float64)
	// NewGauge registers a new gauge metrics. It doesn't track the last value for the metrics.
	NewGauge(name, desc string)

	// IncrementCounter will increase the specified counter metrics by 1.
	IncrementCounter(ctx context.Context, name string, labels ...string)
	// DeltaUpDownCounter increases or decreases the last value with the value specified.
	DeltaUpDownCounter(ctx context.Context, name string, value float64, labels ...string)
	// RecordHistogram gets the value and increase the value in the respective buckets.
	RecordHistogram(ctx context.Context, name string, value float64, labels ...string)
	// SetGauge gets the value and sets the metric to the specified value.
	SetGauge(name string, value float64)
}

type Logger interface {
	Error(args ...interface{})
	Errorf(format string, args ...interface{})
}

type metricsManager struct {
	meter  metric.Meter
	store  Store
	logger Logger
}

func NewMetricManager(meter metric.Meter, logger Logger) Manager {
	return &metricsManager{
		meter:  meter,
		store:  newOtelStore(),
		logger: logger,
	}
}

// Not checking the name or desc parameter because the OTEL package already takes care of the mandatory params
// and return the error.

func (m *metricsManager) NewCounter(name, desc string) {
	counter, err := m.meter.Int64Counter(name, metric.WithDescription(desc))
	if err != nil {
		m.logger.Error(err)

		return
	}

	err = m.store.setCounter(name, counter)
	if err != nil {
		m.logger.Error(err)
	}
}

func (m *metricsManager) NewUpDownCounter(name, desc string) {
	upDownCounter, err := m.meter.Float64UpDownCounter(name, metric.WithDescription(desc))
	if err != nil {
		m.logger.Error(err)

		return
	}

	err = m.store.setUpDownCounter(name, upDownCounter)
	if err != nil {
		m.logger.Error(err)
	}
}

func (m *metricsManager) NewHistogram(name, desc string, buckets ...float64) {
	histogram, err := m.meter.Float64Histogram(name, metric.WithDescription(desc),
		metric.WithExplicitBucketBoundaries(buckets...))
	if err != nil {
		m.logger.Error(err)

		return
	}

	err = m.store.setHistogram(name, histogram)
	if err != nil {
		m.logger.Error(err)
	}
}

func (m *metricsManager) NewGauge(name, desc string) {
	gauge, err := m.meter.Float64ObservableGauge(name, metric.WithDescription(desc))
	if err != nil {
		m.logger.Error(err)

		return
	}

	err = m.store.setGauge(name, gauge)
	if err != nil {
		m.logger.Error(err)
	}
}

func (m *metricsManager) IncrementCounter(ctx context.Context, name string, labels ...string) {
	counter, err := m.store.getCounter(name)
	if err != nil {
		m.logger.Error(err)

		return
	}

	counter.Add(ctx, 1, metric.WithAttributes(getAttributes(labels...)...))
}

func (m *metricsManager) DeltaUpDownCounter(ctx context.Context, name string, value float64, labels ...string) {
	upDownCounter, err := m.store.getUpDownCounter(name)
	if err != nil {
		m.logger.Error(err)

		return
	}

	upDownCounter.Add(ctx, value, metric.WithAttributes(getAttributes(labels...)...))
}

func (m *metricsManager) RecordHistogram(ctx context.Context, name string, value float64, labels ...string) {
	histogram, err := m.store.getHistogram(name)
	if err != nil {
		m.logger.Error(err)

		return
	}

	histogram.Record(ctx, value, metric.WithAttributes(getAttributes(labels...)...))
}

func (m *metricsManager) SetGauge(name string, value float64) {
	gauge, err := m.store.getGauge(name)
	if err != nil {
		m.logger.Error(err)

		return
	}

	_, err = m.meter.RegisterCallback(callbackFunc(gauge, value), gauge)
	if err != nil {
		m.logger.Error(err)
	}
}

func callbackFunc(name metric.Float64ObservableGauge, field float64) func(_ context.Context, o metric.Observer) error {
	return func(_ context.Context, o metric.Observer) error {
		o.ObserveFloat64(name, field)

		return nil
	}
}

func getAttributes(labels ...string) []attribute.KeyValue {
	// TODO - add checks for labelsCount and add warn logs:
	// 1. should always be even as it contains pairs of label key and value.
	// 2. should not exceed 20 to control cardinality
	var attributes []attribute.KeyValue

	if labels != nil {
		for i := 0; i < len(labels); i += 2 {
			attributes = append(attributes, attribute.String(labels[i], labels[i+1]))
		}
	}

	return attributes
}
