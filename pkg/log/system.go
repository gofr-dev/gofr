package log

import (
	"os"
	"runtime"
)

//nolint:gochecknoglobals // the usage of the global variable is required
var hostname string

func fetchSystemStats() map[string]interface{} {
	var m runtime.MemStats

	runtime.ReadMemStats(&m)

	if hostname == "" {
		hostname, _ = os.Hostname()
	}

	stats := make(map[string]interface{})
	stats["alloc"] = m.Alloc
	stats["totalAlloc"] = m.TotalAlloc
	stats["sys"] = m.Sys
	stats["numGC"] = m.NumGC
	stats["goRoutines"] = runtime.NumGoroutine()
	stats["host"] = hostname
	stats["dataCenter"] = os.Getenv("DATA_CENTER")

	if stats["dataCenter"] == "" {
		stats["dataCenter"] = "NOTSET"
	}

	return stats
}
