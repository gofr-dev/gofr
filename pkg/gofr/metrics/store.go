package metrics

import (
	"errors"

	"go.opentelemetry.io/otel/metric"
)

var (
	errMetricDoesNotExist = errors.New("metrics with given name does not exists")
	errMetricAlreadyExist = errors.New("metrics with given name already exists")
)

type MetricStore interface {
	GetCounter(name string) (metric.Int64Counter, error)
	GetUpDownCounter(name string) (metric.Float64UpDownCounter, error)
	GetHistogram(name string) (metric.Float64Histogram, error)
	GetGauge(name string) (metric.Float64ObservableGauge, error)

	setCounter(name string, m metric.Int64Counter) error
	setUpDownCounter(name string, m metric.Float64UpDownCounter) error
	setHistogram(name string, m metric.Float64Histogram) error
	setGauge(name string, m metric.Float64ObservableGauge) error
}

type store struct {
	counter       map[string]metric.Int64Counter
	upDownCounter map[string]metric.Float64UpDownCounter
	histogram     map[string]metric.Float64Histogram
	gauge         map[string]metric.Float64ObservableGauge
}

func (s store) GetCounter(name string) (metric.Int64Counter, error) {
	m, ok := s.counter[name]
	if !ok {
		return nil, errMetricDoesNotExist
	}

	return m, nil
}

func (s store) GetUpDownCounter(name string) (metric.Float64UpDownCounter, error) {
	m, ok := s.upDownCounter[name]
	if !ok {
		return nil, errMetricDoesNotExist
	}

	return m, nil
}

func (s store) GetHistogram(name string) (metric.Float64Histogram, error) {
	m, ok := s.histogram[name]
	if !ok {
		return nil, errMetricDoesNotExist
	}

	return m, nil
}

func (s store) GetGauge(name string) (metric.Float64ObservableGauge, error) {
	m, ok := s.gauge[name]
	if !ok {
		return nil, errMetricDoesNotExist
	}

	return m, nil
}

func (s store) setCounter(name string, m metric.Int64Counter) error {
	_, ok := s.counter[name]
	if !ok {
		s.counter[name] = m

		return nil
	}

	return errMetricAlreadyExist
}

func (s store) setUpDownCounter(name string, m metric.Float64UpDownCounter) error {
	_, ok := s.upDownCounter[name]
	if !ok {
		s.upDownCounter[name] = m

		return nil
	}

	return errMetricAlreadyExist
}

func (s store) setHistogram(name string, m metric.Float64Histogram) error {
	_, ok := s.histogram[name]
	if !ok {
		s.histogram[name] = m

		return nil
	}

	return errMetricAlreadyExist
}

func (s store) setGauge(name string, m metric.Float64ObservableGauge) error {
	_, ok := s.gauge[name]
	if !ok {
		s.gauge[name] = m

		return nil
	}

	return errMetricAlreadyExist
}

func newStore() MetricStore {
	return store{
		counter:       make(map[string]metric.Int64Counter),
		upDownCounter: make(map[string]metric.Float64UpDownCounter),
		histogram:     make(map[string]metric.Float64Histogram),
		gauge:         make(map[string]metric.Float64ObservableGauge),
	}
}
