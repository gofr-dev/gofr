package metrics

import (
	"sync"

	"go.opentelemetry.io/otel/metric"
)

type store struct {
	mu            sync.RWMutex
	counter       map[string]metric.Int64Counter
	upDownCounter map[string]metric.Float64UpDownCounter
	histogram     map[string]metric.Float64Histogram
	gauge         map[string]*float64Gauge
}

// Store represents a store for registered metrics. It provides methods to retrieve and manage different
// types of metrics (counters, up-down counters, histograms, and gauges).
type Store interface {
	getCounter(name string) (metric.Int64Counter, error)
	getUpDownCounter(name string) (metric.Float64UpDownCounter, error)
	getHistogram(name string) (metric.Float64Histogram, error)
	getGauge(name string) (*float64Gauge, error)
	setCounter(name string, m metric.Int64Counter) error
	setUpDownCounter(name string, m metric.Float64UpDownCounter) error
	setHistogram(name string, m metric.Float64Histogram) error
	setGauge(name string, m *float64Gauge) error
}

func newOtelStore() Store {
	return &store{
		counter:       make(map[string]metric.Int64Counter),
		upDownCounter: make(map[string]metric.Float64UpDownCounter),
		histogram:     make(map[string]metric.Float64Histogram),
		gauge:         make(map[string]*float64Gauge),
	}
}

func (s *store) getCounter(name string) (metric.Int64Counter, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	m, ok := s.counter[name]
	if !ok {
		return nil, metricsNotRegistered{metricsName: name}
	}

	return m, nil
}

func (s *store) getUpDownCounter(name string) (metric.Float64UpDownCounter, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	m, ok := s.upDownCounter[name]
	if !ok {
		return nil, metricsNotRegistered{metricsName: name}
	}

	return m, nil
}

func (s *store) getHistogram(name string) (metric.Float64Histogram, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	m, ok := s.histogram[name]
	if !ok {
		return nil, metricsNotRegistered{metricsName: name}
	}

	return m, nil
}

func (s *store) getGauge(name string) (*float64Gauge, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	m, ok := s.gauge[name]
	if !ok {
		return m, metricsNotRegistered{metricsName: name}
	}

	return m, nil
}

func (s *store) setCounter(name string, m metric.Int64Counter) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	_, ok := s.counter[name]
	if !ok {
		s.counter[name] = m

		return nil
	}

	return metricsAlreadyRegistered{metricsName: name}
}

func (s *store) setUpDownCounter(name string, m metric.Float64UpDownCounter) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	_, ok := s.upDownCounter[name]
	if !ok {
		s.upDownCounter[name] = m

		return nil
	}

	return metricsAlreadyRegistered{metricsName: name}
}

func (s *store) setHistogram(name string, m metric.Float64Histogram) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	_, ok := s.histogram[name]
	if !ok {
		s.histogram[name] = m

		return nil
	}

	return metricsAlreadyRegistered{metricsName: name}
}

func (s *store) setGauge(name string, m *float64Gauge) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	_, ok := s.gauge[name]
	if !ok {
		s.gauge[name] = m

		return nil
	}

	return metricsAlreadyRegistered{metricsName: name}
}
