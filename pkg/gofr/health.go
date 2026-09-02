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
	return healthResponse{Name: c.GetAppName(), Status: aggregateStatus(c)}, nil
}

// aggregateStatus runs the full health check and keeps only the overall status, discarding every
// per-dependency detail. The aggregation in Container.appHealth yields "UP" when all dependencies
// are healthy and "DEGRADED" when one or more are down; "DOWN" is a fail-closed default for the
// unreachable case where the aggregate key is missing or not a string.
//
// This stays unexported here rather than being a method on Container: Context embeds
// *container.Container, so an exported method would be promoted onto every handler's ctx, and a
// per-request call sweeps every datasource unless HEALTH_CACHE_TTL is set, which it is not by
// default.
func aggregateStatus(c *Context) string {
	const statusDown = "DOWN"

	m, ok := c.Health(c).(map[string]any)
	if !ok {
		return statusDown
	}

	status, ok := m["status"].(string)
	if !ok {
		return statusDown
	}

	return status
}
