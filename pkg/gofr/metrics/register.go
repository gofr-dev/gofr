package metrics

import (
	"context"
	"sync"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
)

// Error can also be returned from all the methods, but it is decided not to do so such that to keep the usage clean -
// as any errors are already being logged from here. Otherwise, user would need to check the error every time.

// Manager defines the interface for registering and interacting with different types of metrics
// (counters, up-down counters, histograms, and gauges).
type Manager interface {
	NewCounter(name, desc string)
	NewUpDownCounter(name, desc string)
	NewHistogram(name, desc string, buckets ...float64)
	NewGauge(name, desc string)

	IncrementCounter(ctx context.Context, name string, labels ...string)
	DeltaUpDownCounter(ctx context.Context, name string, value float64, labels ...string)
	RecordHistogram(ctx context.Context, name string, value float64, labels ...string)
	SetGauge(name string, value float64, labels ...string)
}

// Logger defines a simple interface for logging messages at different log levels.
type Logger interface {
	Error(args ...any)
	Errorf(format string, args ...any)
	Warn(args ...any)
	Warnf(format string, args ...any)
}

type metricsManager struct {
	meter  metric.Meter
	store  Store
	logger Logger
}

// Developer Note: float64Gauge is used instead of metric.Float64ObservableGauge because we need a synchronous gauge metric
// and otel/metric supports only asynchronous gauge (Float64ObservableGauge).
// And if we use the otel/metric, we would not be able to have support for labels, Hence created a custom type to implement it.
type float64Gauge struct {
	observations map[attribute.Set]float64
	mu           sync.RWMutex
}

// NewMetricsManager creates a new metrics manager instance with the provided metric  meter and logger.
func NewMetricsManager(meter metric.Meter, logger Logger) Manager {
	return &metricsManager{
		meter:  meter,
		store:  newOtelStore(),
		logger: logger,
	}
}

// Developer Note : we are not checking the name or desc parameter because the OTEL
// package already takes care of the mandatory params and returns the error.

// NewCounter registers a new counter metrics whose values are monotonically increasing
// and cannot decrement.
//
//	Usage: m.NewCounter("requests_total", "Total number of requests")
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

// NewUpDownCounter registers a new UpDown Counter metrics.
//
//	Usage:
//	 m.NewUpDownCounter("active_users", "Number of active users")
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

// NewHistogram registers a new histogram metrics with different buckets.
//
//	Usage:
//	 m.NewHistogram("another_histogram", "Another histogram metric", 0, 10, 100, 1000)
//
// When creating a histogram metric, we can specify custom bucket boundaries to group data points
// into ranges. Buckets represent specific ranges of values. Each value recorded in the histogram
// is placed into the appropriate bucket based on its value compared to the bucket boundaries.
//
//	For example, when tracking response times in milliseconds, we might define buckets like [0, 10),
//	[10, 100), [100, 1000), [1000, +Inf), where each range represents response times
//	within a certain range, and the last bucket includes all values above 1000ms (represented by +Inf,
//	which stands for positive infinity).
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

// NewGauge registers a new gauge metrics. This metric can set
// the value of metric to a particular value, but it doesn't store the last recorded value for the metrics.
//
//	Usage:
//	m.NewGauge("memory_usage", "Current memory usage in bytes")
func (m *metricsManager) NewGauge(name, desc string) {
	gauge := &float64Gauge{observations: make(map[attribute.Set]float64)}

	_, err := m.meter.Float64ObservableGauge(name, metric.WithDescription(desc), metric.WithFloat64Callback(gauge.callbackFunc))
	if err != nil {
		m.logger.Error(err)

		return
	}

	err = m.store.setGauge(name, gauge)
	if err != nil {
		m.logger.Error(err)
	}
}

// callbackFunc implements the callback function for the underlying asynchronous gauge
// it observes the current state of all previous set() calls.
func (f *float64Gauge) callbackFunc(_ context.Context, o metric.Float64Observer) error {
	f.mu.RLock()
	defer f.mu.RUnlock()

	for attrs, val := range f.observations {
		o.Observe(val, metric.WithAttributeSet(attrs))
	}

	return nil
}

// IncrementCounter increases the specified registered counter metric by 1.
//
//	Usage:
//
//	    // Increment a counter metric without labels
//	 1. m.IncrementCounter(ctx, "example_counter")
//
//	    // Increment a counter metric with labels
//	 2. m.IncrementCounter(ctx, "example_counter_with_labels", "label1", "value1", "label2", "value2")
//
// The IncrementCounter method is used to increase the specified counter metric by 1. If the counter metric
// does not exist, an error is logged. Optionally, we can provide labels to associate additional information
// with the counter metric. Labels are provided as key-value pairs where the label name and value alternate.
// For example, "label1", "value1", "label2", "value2". Labels allow us to segment and filter your metrics
// based on different dimensions.
func (m *metricsManager) IncrementCounter(ctx context.Context, name string, labels ...string) {
	counter, err := m.store.getCounter(name)
	if err != nil {
		m.logger.Error(err)

		return
	}

	counter.Add(ctx, 1, metric.WithAttributes(m.getAttributes(name, labels...)...))
}

// DeltaUpDownCounter increases or decreases the last value with the value specified.
//
//	Usage:
//
//	   // Increase the number of active users by 1.5 without any additional labels
//	1. m.DeltaUpDownCounter(ctx, "active_users", 1.5)
//
//	   // Increase the number of successful logins by 1.5 with labels.
//	2. m.DeltaUpDownCounter(ctx, "successful_logins", 1.5, "label1", "value1", "label2", "value2")
//
// The DeltaUpDownCounter method is used to increase or decrease the last value of the specified UpDown counter metric
// by the given value. For example, we might use this method to track changes in the number of active users or
// successful login attempts. Labels can provide additional context, such as the method and endpoint of the request,
// allowing us to analyze metrics based on different dimensions.
func (m *metricsManager) DeltaUpDownCounter(ctx context.Context, name string, value float64, labels ...string) {
	upDownCounter, err := m.store.getUpDownCounter(name)
	if err != nil {
		m.logger.Error(err)

		return
	}

	upDownCounter.Add(ctx, value, metric.WithAttributes(m.getAttributes(name, labels...)...))
}

// RecordHistogram records the specified value in the respective buckets of the histogram metric.
//
//	Usage:
//
//	    // Record the latency of an API request without any  labels.
//	 1. m.RecordHistogram(ctx, "api_request_latency", 25.5)
//
//	    // Record the latency of an API request with labels.
//	 2. m.RecordHistogram(ctx, "api_request_latency", 25.5, "label1", "value1", "label2", "value2")
func (m *metricsManager) RecordHistogram(ctx context.Context, name string, value float64, labels ...string) {
	histogram, err := m.store.getHistogram(name)
	if err != nil {
		m.logger.Error(err)

		return
	}

	histogram.Record(ctx, value, metric.WithAttributes(m.getAttributes(name, labels...)...))
}

// SetGauge gets the value and sets the metric to the specified value.
// Unlike counters, gauges do not track the last value for the metric. This method allows us to
// directly set the value of the gauge to the specified value.
//
//	Usage:
//	manager.SetGauge("memory_usage", 1024*1024*100)
//	// Set memory usage to 100 MB
func (m *metricsManager) SetGauge(name string, value float64, labels ...string) {
	gauge, err := m.store.getGauge(name)
	if err != nil {
		m.logger.Error(err)

		return
	}

	gauge.set(value, attribute.NewSet(m.getAttributes(name, labels...)...))
}

func (f *float64Gauge) set(val float64, attrs attribute.Set) {
	f.mu.Lock()
	defer f.mu.Unlock()

	f.observations[attrs] = val
}

// getAttributes validates the given labels and convert them to corresponding otel attributes.
func (m *metricsManager) getAttributes(name string, labels ...string) []attribute.KeyValue {
	labelsCount := len(labels)
	if labelsCount%2 != 0 {
		m.logger.Warnf("metrics %v label has invalid key-value pairs", name)
	}

	cardinalityLimit := 20
	if labelsCount > cardinalityLimit {
		m.logger.Warnf("metrics %v has high cardinality: %v", name, labelsCount)
	}

	var attributes []attribute.KeyValue

	if labels != nil {
		for i := 0; i < len(labels)-1; i += 2 {
			attributes = append(attributes, attribute.String(labels[i], labels[i+1]))
		}
	}

	return attributes
}
