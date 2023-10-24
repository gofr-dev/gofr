package metrics

import (
	"github.com/prometheus/client_golang/prometheus"

	"gofr.dev/pkg/errors"
)

const (
	// metricErr raised if metric is invalid or already registered
	metricErr      = errors.Error("invalid/duplicate metrics collector registration attempted")
	metricNotFound = errors.Error("metric with given name is not registered")
)

type promVec struct {
	counter   map[string]*prometheus.CounterVec
	histogram map[string]*prometheus.HistogramVec
	gauge     map[string]*prometheus.GaugeVec
	summary   map[string]*prometheus.SummaryVec
}

func newPromVec() *promVec {
	return &promVec{
		counter:   make(map[string]*prometheus.CounterVec),
		histogram: make(map[string]*prometheus.HistogramVec),
		gauge:     make(map[string]*prometheus.GaugeVec),
		summary:   make(map[string]*prometheus.SummaryVec),
	}
}

// IncCounter increments a Prometheus counter metric by name and labels.
func (p *promVec) IncCounter(name string, labels ...string) error {
	counter, ok := p.counter[name]
	if !ok {
		return metricNotFound
	}

	c, err := counter.GetMetricWithLabelValues(labels...)
	if err != nil {
		return err
	}

	c.Inc()

	return nil
}

// AddCounter adds a value to a Prometheus counter metric by name and labels.
func (p *promVec) AddCounter(name string, val float64, labels ...string) error {
	counter, ok := p.counter[name]
	if !ok {
		return metricNotFound
	}

	c, err := counter.GetMetricWithLabelValues(labels...)
	if err != nil {
		return err
	}

	c.Add(val)

	return nil
}

// ObserveHistogram observes a Prometheus histogram metric by name and labels.
func (p *promVec) ObserveHistogram(name string, val float64, labels ...string) error {
	histogram, ok := p.histogram[name]
	if !ok {
		return metricNotFound
	}

	observer, err := histogram.GetMetricWithLabelValues(labels...)
	if err != nil {
		return err
	}

	observer.Observe(val)

	return nil
}

// SetGauge sets the value of a Prometheus gauge metric by name and labels.
func (p *promVec) SetGauge(name string, val float64, labels ...string) error {
	gauge, ok := p.gauge[name]
	if !ok {
		return metricNotFound
	}

	g, err := gauge.GetMetricWithLabelValues(labels...)
	if err != nil {
		return err
	}

	g.Set(val)

	return nil
}

// ObserveSummary observes a Prometheus summary metric by name and labels.
func (p *promVec) ObserveSummary(name string, val float64, labels ...string) error {
	summary, ok := p.summary[name]
	if !ok {
		return metricNotFound
	}

	observer, err := summary.GetMetricWithLabelValues(labels...)
	if err != nil {
		return err
	}

	observer.Observe(val)

	return nil
}

func (p *promVec) registerCounter(name, help string, labels ...string) error {
	counter := prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: name,
			Help: help,
		}, labels)

	err := prometheus.Register(counter)
	if err != nil {
		return metricErr
	}

	p.counter[name] = counter

	return nil
}

func (p *promVec) registerHistogram(name, help string, buckets []float64, labels ...string) error {
	histogram := prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    name,
			Help:    help,
			Buckets: buckets,
		}, labels)

	err := prometheus.Register(histogram)
	if err != nil {
		return metricErr
	}

	p.histogram[name] = histogram

	return nil
}

func (p *promVec) registerGauge(name, help string, labels ...string) error {
	gauge := prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: name,
			Help: help,
		}, labels)

	err := prometheus.Register(gauge)
	if err != nil {
		return metricErr
	}

	p.gauge[name] = gauge

	return nil
}

func (p *promVec) registerSummary(name, help string, labels ...string) error {
	summary := prometheus.NewSummaryVec(
		prometheus.SummaryOpts{
			Name: name,
			Help: help,
		}, labels)

	err := prometheus.Register(summary)
	if err != nil {
		return metricErr
	}

	p.summary[name] = summary

	return nil
}
