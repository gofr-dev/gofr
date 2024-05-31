// Package metrics provides functionalities for instrumenting GoFr applications with metrics.
package metrics

import "fmt"

type metricsAlreadyRegistered struct {
	metricsName string
}

type metricsNotRegistered struct {
	metricsName string
}

func (e metricsAlreadyRegistered) Error() string {
	return fmt.Sprintf("Metrics %v already registered", e.metricsName)
}

func (e metricsNotRegistered) Error() string {
	return fmt.Sprintf("Metrics %v is not registered", e.metricsName)
}
