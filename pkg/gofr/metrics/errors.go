package metrics

import "fmt"

type metricAlreadyRegistered struct {
	metricsName string
}

type metricNotRegistered struct {
	metricsName string
}

func (e metricAlreadyRegistered) Error() string {
	return fmt.Sprintf("Metrics %v already registered", e.metricsName)
}

func (e metricNotRegistered) Error() string {
	return fmt.Sprintf("Metrics %v is not registered", e.metricsName)
}
