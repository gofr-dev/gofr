package service

import "sync"

var (
	//nolint:gochecknoglobals // Global map to track registered metrics
	registeredMetrics = make(map[string]bool)
	//nolint:gochecknoglobals // Mutex to protect the global map
	metricsMu sync.Mutex
)

func registerCounter(m Metrics, name, desc string) {
	metricsMu.Lock()
	defer metricsMu.Unlock()

	if registeredMetrics[name] {
		return
	}

	m.NewCounter(name, desc)
	registeredMetrics[name] = true
}
