package metrics

import (
	"context"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
)

// Error can also be returned from all the methods, but it is decided not to do so such that to keep the usage clean -
// as any errors are already being logged from here. Otherwise, user would need to check the error everytime.

type Manager interface {
	NewCounter(name, desc string)
	NewUpDownCounter(name, desc string)
	NewHistogram(name, desc string, buckets ...float64)
	NewGauge(name, desc string)

	IncrementCounter(ctx context.Context, name string, labels ...string)
	DeltaUpDownCounter(ctx context.Context, name string, value float64, labels ...string)
	RecordHistogram(ctx context.Context, name string, value float64, labels ...string)
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

// Developer Note : we are not checking the name or desc parameter because the OTEL
// package already takes care of the mandatory params and returns the error.

// NewCounter registers a new counter metrics. It can not be reduced.
//
// If an error occurs during the registration process, such as if
// the metric with the same name already exists or if there are issues with metric creation, an error
// message is logged using the provided logger.
//
// Usage:
// m.NewCounter("requests_total", "Total number of requests")
//
// Example:
// m.IncrementCounter(ctx, "requests_total") // increments total counter by 1.
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
// If an error occurs during the registration process, such as if
// the metric with the same name already exists or if there are issues with metric creation, an error
// message is logged using the provided logger.
//
// Usage:
//
// m.NewUpDownCounter("active_users", "Number of active users")
//
// Example:
//
// Once registered, you can use the UpDown counter to track changes in values. For instance:
// m.DeltaUpDownCounter(ctx, "active_users", 10) // Increase active users by 10.
// m.DeltaUpDownCounter(ctx, "active_users", -10) // Decrease active users by 10.
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
// If an error occurs during the registration process, such as if
// the metric with the same name already exists or if there are issues with metric creation, an error
// message is logged using the provided logger.
//
// Usage:
//
//	m.NewHistogram("another_histogram", "Another histogram metric", 0, 10, 100, 1000)
//
// When creating a histogram metric, you can specify custom bucket boundaries to group data points
// into ranges. Buckets represent specific ranges of values. Each value recorded in the histogram
// is placed into the appropriate bucket based on its value compared to the bucket boundaries.
//
// For example, when tracking response times in milliseconds, you might define buckets like [0, 10),
// [10, 100), [100, 1000), [1000, +Inf), where each range represents response times
// within a certain range, and the last bucket includes all values above 1000ms (represented by +Inf,
// which stands for positive infinity).
//
// Example:
//
// Once registered, you can record values to the histogram
// m.RecordHistogram(ctx, "response_times", 15) // Record a response time of 15ms.
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

// NewGauge registers a new gauge metrics. It doesn't track the last value for the metrics.
//
// If an error occurs during the registration process, such as if
// the metric with the same name already exists or if there are issues with metric creation, an error
// message is logged using the provided logger.
//
// Usage:
//
// m.NewGauge("memory_usage", "Current memory usage in bytes")
//
// Example:
//
// Once registered, you can use the gauge to track instantaneous values. For instance:
// m.SetGauge("memory_usage", 1024*1024*100) // Set memory usage to 100 MB.
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

// IncrementCounter increases the specified registered counter metric by 1.
//
// Usage:
//
// Increment a counter metric without labels
// m.IncrementCounter(ctx, "example_counter")
//
// Increment a counter metric with labels
// m.IncrementCounter(ctx, "example_counter_with_labels", "label1", "value1", "label2", "value2")
//
// The IncrementCounter method is used to increase the specified counter metric by 1. If the counter metric
// does not exist, an error is logged. Optionally, you can provide labels to associate additional information
// with the counter metric. Labels are provided as key-value pairs where the label name and value alternate.
// For example, "label1", "value1", "label2", "value2". Labels allow you to segment and filter your metrics
// based on different dimensions.
func (m *metricsManager) IncrementCounter(ctx context.Context, name string, labels ...string) {
	counter, err := m.store.getCounter(name)
	if err != nil {
		m.logger.Error(err)

		return
	}

	counter.Add(ctx, 1, metric.WithAttributes(getAttributes(labels...)...))
}

// DeltaUpDownCounter increases or decreases the last value with the value specified.
//
// Usage:
//
// Increase the number of active users by 1.5 without any additional labels
// m.DeltaUpDownCounter(ctx, "active_users", 1.5)
//
// Increase the number of successful logins by 1.5 with labels.
// m.DeltaUpDownCounter(ctx, "successful_logins", 1.5, "label1", "value1", "label2", "value2")
//
// The DeltaUpDownCounter method is used to increase or decrease the last value of the specified UpDown counter metric
// by the given value. For example, you might use this method to track changes in the number of active users or
// successful login attempts. Labels can provide additional context, such as the method and endpoint of the request,
// allowing you to analyze metrics based on different dimensions.
func (m *metricsManager) DeltaUpDownCounter(ctx context.Context, name string, value float64, labels ...string) {
	upDownCounter, err := m.store.getUpDownCounter(name)
	if err != nil {
		m.logger.Error(err)

		return
	}

	upDownCounter.Add(ctx, value, metric.WithAttributes(getAttributes(labels...)...))
}

// RecordHistogram records the specified value in the respective buckets of the histogram metric.
//
// Usage:
//
// m.RecordHistogram(ctx, "api_request_latency", 25.5) // Record the latency of an API request without any  labels.
//
// m.RecordHistogram(ctx, "api_request_latency", 25.5, "label1", "value1", "label2", "value2") // Record the latency of
// an API request with labels.
func (m *metricsManager) RecordHistogram(ctx context.Context, name string, value float64, labels ...string) {
	histogram, err := m.store.getHistogram(name)
	if err != nil {
		m.logger.Error(err)

		return
	}

	histogram.Record(ctx, value, metric.WithAttributes(getAttributes(labels...)...))
}

// SetGauge gets the value and sets the metric to the specified value.
// Unlike counters, gauges do not track the last value for the metric. This method allows you to
// directly set the value of the gauge to the specified value.
//
// Usage:
// manager.SetGauge("memory_usage", 1024*1024*100) // Set memory usage to 100 MB
//
// If the gauge metric does not exist,  an error message is logged using the provided logger.
// If an error occurs during the setting  process, such as if the metric with the same name does not exist
// or if there are issues with the callback registration, an error message is also logged.
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
