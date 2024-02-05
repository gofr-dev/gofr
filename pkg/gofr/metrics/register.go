package metrics

import (
	"context"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
)

type Manager interface {
	NewCounter(name, desc string) error
	NewUpDownCounter(name, desc string) error
	NewHistogram(name, desc string, buckets ...float64) error
	NewGauge(name, desc string) error

	IncrementCounter(ctx context.Context, name string, labels ...string) error
	DeltaUpDownCounter(ctx context.Context, name string, value float64, labels ...string) error
	RecordHistogram(ctx context.Context, name string, value float64, labels ...string) error
	SetGauge(name string, value float64) error
}

type metricsManager struct {
	meter metric.Meter
	store store
}

func NewMetricManager(meter metric.Meter) Manager {
	return &metricsManager{
		meter: meter,
		store: newOtelStore(),
	}
}

func (m *metricsManager) NewCounter(name, desc string) error {
	counter, err := m.meter.Int64Counter(name, metric.WithDescription(desc))
	if err != nil {
		return err
	}

	err = m.store.setCounter(name, counter)
	if err != nil {
		return err
	}

	return nil
}

func (m *metricsManager) NewUpDownCounter(name, desc string) error {
	upDownCounter, err := m.meter.Float64UpDownCounter(name, metric.WithDescription(desc))
	if err != nil {
		return err
	}

	err = m.store.setUpDownCounter(name, upDownCounter)
	if err != nil {
		return err
	}

	return nil
}

func (m *metricsManager) NewHistogram(name, desc string, buckets ...float64) error {
	histogram, err := m.meter.Float64Histogram(name, metric.WithDescription(desc),
		metric.WithExplicitBucketBoundaries(buckets...))
	if err != nil {
		return err
	}

	err = m.store.setHistogram(name, histogram)
	if err != nil {
		return err
	}

	return nil
}

func (m *metricsManager) NewGauge(name, desc string) error {
	gauge, err := m.meter.Float64ObservableGauge(name, metric.WithDescription(desc))
	if err != nil {
		return err
	}

	err = m.store.setGauge(name, gauge)
	if err != nil {
		return err
	}

	return nil
}

func (m *metricsManager) IncrementCounter(ctx context.Context, name string, labels ...string) error {
	counter, err := m.store.getCounter(name)
	if err != nil {
		return errMetricDoesNotExist
	}

	counter.Add(ctx, 1, metric.WithAttributes(getAttributes(labels...)...))

	return nil
}

func (m *metricsManager) DeltaUpDownCounter(ctx context.Context, name string, value float64, labels ...string) error {
	upDownCounter, err := m.store.getUpDownCounter(name)
	if err != nil {
		return errMetricDoesNotExist
	}

	upDownCounter.Add(context.Background(), value, metric.WithAttributes(getAttributes(labels...)...))

	return nil
}

func (m *metricsManager) RecordHistogram(ctx context.Context, name string, value float64, labels ...string) error {
	histogram, err := m.store.getHistogram(name)
	if err != nil {
		return errMetricDoesNotExist
	}

	histogram.Record(context.Background(), value, metric.WithAttributes(getAttributes(labels...)...))

	return nil
}

func (m *metricsManager) SetGauge(name string, value float64) error {
	gauge, err := m.store.getGauge(name)
	if err != nil {
		return err
	}

	_, err = m.meter.RegisterCallback(callbackFunc(gauge, value), gauge)
	if err != nil {
		return err
	}

	return nil
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
