package gofr

import (
	"context"
	"net/http"

	"gofr.dev/pkg/gofr/container"
	"gofr.dev/pkg/gofr/logging"
)

const (
	statusUp   = "UP"
	statusDown = "DOWN"
)

// HealthCheckFunc lets an application decide the readiness reported by the public, unauthenticated
// /.well-known/health endpoint. It receives the request context and the container — so critical
// datasources such as c.Redis or c.SQL can be probed — and returns the status string to report in
// the response body along with the HTTP status code to respond with. For example, return
// ("UP", http.StatusOK) when ready, or ("DOWN", http.StatusServiceUnavailable) when a critical
// dependency is not yet available so the pod is kept out of service until it is.
type HealthCheckFunc func(ctx context.Context, c *container.Container) (status string, statusCode int)

// SetHealthCheck registers an optional readiness closure for the public /.well-known/health
// endpoint. Without it, the endpoint responds 200 with the aggregate {name, status} of all
// registered dependencies, preserving existing behavior. Either way the endpoint never exposes
// per-dependency details (hosts, ports, credentials, connection stats) — that information is
// intentionally kept off the unauthenticated port.
func (a *App) SetHealthCheck(fn HealthCheckFunc) {
	a.healthCheck = fn
}

// healthResponse is the minimal, non-sensitive body returned by /.well-known/health. It carries
// only the application name and the aggregate status — never per-dependency inventory.
type healthResponse struct {
	Name   string `json:"name"`
	Status string `json:"status"`
}

// errHealthNotReady carries a caller-chosen HTTP status code for a not-ready readiness result so
// the responder emits that code instead of the default 200. Its message is the status string, so
// the body still conveys the reported status without leaking any dependency details.
type errHealthNotReady struct {
	status string
	code   int
}

func (e errHealthNotReady) Error() string   { return e.status }
func (e errHealthNotReady) StatusCode() int { return e.code }

// LogLevel keeps a not-ready readiness at WARN rather than ERROR. A 503 is expected during startup
// and rolling deploys — Kubernetes probes it repeatedly — so it should not flood logs as an error.
func (errHealthNotReady) LogLevel() logging.Level { return logging.WARN }

var _ logging.LogLevelResponder = errHealthNotReady{}

// healthHandler serves the public, unauthenticated /.well-known/health endpoint. It reports only
// the application name and aggregate status — no hosts, ports, credentials, or connection stats.
// No HTTP route serves the full detailed map; Container.Health still computes it for in-process ops
// tooling, and #3806 tracks exposing it on the metrics server (METRICS_PORT), behind the same
// network boundary as /metrics and /debug/pprof.
func (a *App) healthHandler(c *Context) (any, error) {
	status, code := a.readiness(c)

	body := healthResponse{Name: c.GetAppName(), Status: status}

	if code == http.StatusOK {
		return body, nil
	}

	return nil, errHealthNotReady{status: status, code: code}
}

// readiness resolves the status string and HTTP status code for the health endpoint. It delegates
// to the app-provided closure when one is set, and otherwise falls back to the aggregate dependency
// status served with a 200 — matching the pre-existing default behavior.
func (a *App) readiness(c *Context) (status string, code int) {
	if a.healthCheck == nil {
		return aggregateStatus(c), http.StatusOK
	}

	status, code = a.healthCheck(c.Context, c.Container)

	if code == 0 {
		code = http.StatusOK
	}

	if status == "" {
		status = statusFromCode(code)
	}

	return status, code
}

// statusFromCode derives a status string when the closure returned a code but no status: 200 maps
// to UP and any other code to DOWN, so the body still reports a status consistent with the code.
func statusFromCode(code int) string {
	if code == http.StatusOK {
		return statusUp
	}

	return statusDown
}

// aggregateStatus runs the full health check and keeps only the overall status, discarding every
// per-dependency detail. The aggregation in Container.appHealth yields "UP" when all dependencies
// are healthy and "DEGRADED" when one or more are down; "DOWN" is a fail-closed default for the
// unreachable case where the aggregate key is missing or not a string.
//
// This stays unexported here rather than being a method on Container: Context embeds
// *container.Container, so an exported method would be promoted onto every handler's ctx, and
// Health does no caching — a per-request call would sweep every datasource in the hot path.
func aggregateStatus(c *Context) string {
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
