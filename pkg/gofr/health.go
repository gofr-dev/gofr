package gofr

import (
	"errors"
	"fmt"
	"net/http"

	"gofr.dev/pkg/gofr/logging"
)

const (
	statusUp   = "UP"
	statusDown = "DOWN"
)

// ReadinessCheck decides whether the application is ready to serve traffic, for the public,
// unauthenticated /.well-known/health endpoint. It receives the request context — which carries the
// container, so datasources such as ctx.Redis or ctx.SQL can be probed — and framework, GoFr's own
// verdict on every registered datasource and service: nil when they are all up, non-nil when one or
// more are not.
//
// Returning nil means ready; returning any error means not ready, and the endpoint answers 503 so a
// Kubernetes readiness probe keeps the pod out of service. The returned error is logged but never
// written to the response: the endpoint is unauthenticated and must not leak dependency details.
//
// What the check does with framework is the whole of the policy. Propagate it to require GoFr's
// checks in addition to your own:
//
//	app.SetReadinessCheck(func(ctx *gofr.Context, framework error) error {
//		if framework != nil {
//			return framework
//		}
//
//		return licenseDB.PingContext(ctx)
//	})
//
// Or ignore it — writing _ — to replace GoFr's verdict entirely, which is what readiness that is a
// relationship between dependencies rather than a conjunction of them requires:
//
//	app.SetReadinessCheck(func(ctx *gofr.Context, _ error) error {
//		if redisUp(ctx) || sqlUp(ctx) {
//			return nil
//		}
//
//		return errNoBackingStore
//	})
type ReadinessCheck func(ctx *Context, framework error) error

// SetReadinessCheck registers the application's readiness check for /.well-known/health. Without
// one the endpoint behaves exactly as before — HTTP 200 with the aggregate {name, status}, DEGRADED
// included, since which dependencies are critical is an application decision. Register before
// App.Run; a second call replaces the first.
func (a *App) SetReadinessCheck(check ReadinessCheck) {
	a.readinessCheck = check
}

// healthResponse is the minimal, non-sensitive body returned by /.well-known/health. It carries
// only the application name and the aggregate status — never per-dependency inventory.
type healthResponse struct {
	Name   string `json:"name"`
	Status string `json:"status"`
}

// errNotReady is the only thing a failed readiness reports to the caller: the status string and a
// 503. The reason — which check failed and why — goes to the logs, never to this unauthenticated
// endpoint.
type errNotReady struct{}

func (errNotReady) Error() string   { return statusDown }
func (errNotReady) StatusCode() int { return http.StatusServiceUnavailable }

// LogLevel keeps a not-ready readiness at WARN rather than ERROR. A 503 is expected during startup
// and rolling deploys — Kubernetes probes it repeatedly — so it should not flood logs as an error.
func (errNotReady) LogLevel() logging.Level { return logging.WARN }

var _ logging.LogLevelResponder = errNotReady{}

// errFrameworkChecks marks a not-ready result that came from GoFr's own dependency checks rather
// than from an application check, so the log line says which side failed.
var errFrameworkChecks = errors.New("framework dependency checks")

// healthHandler serves the public, unauthenticated /.well-known/health endpoint. It reports only
// the application name and aggregate status — no hosts, ports, credentials, or connection stats.
// No HTTP route serves the full detailed map; Container.Health still computes it for in-process ops
// tooling, and #3806 tracks exposing it on the metrics server (METRICS_PORT), behind the same
// network boundary as /metrics and /debug/pprof.
func (a *App) healthHandler(c *Context) (any, error) {
	// Default behavior, unchanged: with no application check registered the endpoint reports the
	// aggregate dependency status with a 200 — DEGRADED is informational, not a readiness verdict.
	if a.readinessCheck == nil {
		return healthResponse{Name: c.GetAppName(), Status: aggregateStatus(c)}, nil
	}

	if err := a.readinessCheck(c, frameworkReadiness(c)); err != nil {
		c.Warnf("readiness: not ready: %v", err)

		return nil, errNotReady{}
	}

	return healthResponse{Name: c.GetAppName(), Status: statusUp}, nil
}

// frameworkReadiness renders GoFr's own dependency checks as the error passed to the application's
// check: nil when every registered datasource and service is up, and otherwise an error naming the
// aggregate status — never the per-dependency detail behind it, which must not reach the response
// even by way of a check that returns this error unchanged.
//
// It is evaluated before the check runs, so the datasource sweep happens on every probe even for a
// check that ignores the parameter. That matches the pre-existing default path, which sweeps on
// every probe too.
func frameworkReadiness(c *Context) error {
	if status := aggregateStatus(c); status != statusUp {
		return fmt.Errorf("%w: reported %s", errFrameworkChecks, status)
	}

	return nil
}

// logReadiness states at startup that readiness is no longer the default, so a 503 from this
// endpoint is never a surprise about where the verdict came from.
func (a *App) logReadiness() {
	if a.container.Logger == nil || a.readinessCheck == nil {
		return
	}

	a.container.Logger.Infof("readiness: app check registered - framework checks: enabled, " +
		"propagated to the app check")
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
