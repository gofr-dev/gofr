package metrics

import (
	"context"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
)

type Registrer interface {
	NewCounter(name, desc string) error
	NewUpDownCounter(name, desc string) error
	NewHistogram(name, desc string, buckets ...float64) error
	NewGauge(name, desc string) error
}

type Manager interface {
	Registrer
	Updater
}

type Updater interface {
	IncrementCounter(ctx context.Context, name string, labels ...string) error
}

type metricsManager struct {
	meter metric.Meter
	store MetricStore
}

func NewMetricManager(meter metric.Meter) Manager {
	return &metricsManager{
		meter: meter,
		store: newStore(),
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
	mt, err := m.store.GetCounter(name)
	if err != nil {
		return errMetricDoesNotExist
	}

	mt.Add(ctx, 1, metric.WithAttributes(getAttributes(labels...)...))

	return nil
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
