package gofr

import (
	"context"
	"net/http"

	"gofr.dev/pkg/gofr/container"
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

// healthHandler serves the public, unauthenticated /.well-known/health endpoint. It reports only
// the application name and aggregate status — no hosts, ports, credentials, or connection stats.
// The full detailed health map remains available internally via Container.Health for ops tooling.
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
		return c.HealthStatus(c), http.StatusOK
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

func statusFromCode(code int) string {
	if code == http.StatusOK {
		return statusUp
	}

	return statusDown
}
