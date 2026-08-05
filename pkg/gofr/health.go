package gofr

// healthResponse is the minimal, non-sensitive body returned by /.well-known/health. It carries
// only the application name and the aggregate status — never per-dependency inventory.
type healthResponse struct {
	Name   string `json:"name"`
	Status string `json:"status"`
}

// healthHandler serves the public, unauthenticated /.well-known/health endpoint. It reports only
// the application name and aggregate status — no hosts, ports, credentials, or connection stats.
// No HTTP route serves the full detailed map after this change; Container.Health still computes it
// for in-process ops tooling, and #3806 tracks exposing it on the metrics server (METRICS_PORT),
// behind the same network boundary as /metrics and /debug/pprof.
func healthHandler(c *Context) (any, error) {
	return healthResponse{Name: c.GetAppName(), Status: c.HealthStatus(c)}, nil
}
