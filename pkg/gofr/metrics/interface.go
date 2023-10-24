// Package metrics provides the functionality to create custom metrics for applications
package metrics

// Metric provides support of custom metric
type Metric interface {
	// IncCounter increments the value of counter by one
	IncCounter(name string, labels ...string) error
	// AddCounter adds specified value in counter
	AddCounter(name string, val float64, labels ...string) error
	// ObserveHistogram creates observation of Histogram for specified value
	ObserveHistogram(name string, val float64, labels ...string) error
	// SetGauge sets the specific value in Gauge
	SetGauge(name string, val float64, labels ...string) error
	// ObserveSummary creates observation of Summary for specified value
	ObserveSummary(name string, val float64, labels ...string) error
}
