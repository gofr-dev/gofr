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
// unauthenticated /.well-known/health endpoint. It receives the request context, which carries the
// container, so datasources such as ctx.Redis or ctx.SQL can be probed.
//
// Returning nil means ready; returning any error means not ready, and the endpoint answers 503 so a
// Kubernetes readiness probe keeps the pod out of service. The returned error is logged but never
// written to the response: the endpoint is unauthenticated and must not leak dependency details.
type ReadinessCheck func(ctx *Context) error

// ReadinessOption configures how a readiness check registered with AddReadinessCheck is evaluated.
type ReadinessOption func(*readinessCheck)

// readinessCheck is one registered check together with how it was registered.
type readinessCheck struct {
	check ReadinessCheck

	// replaceFramework records that this check was registered with ReplaceFrameworkChecks, so
	// GoFr's own dependency checks no longer gate readiness — and, since they no longer decide
	// anything, are not run at all.
	replaceFramework bool
}

// ReplaceFrameworkChecks makes the checks registered with it the whole readiness verdict: GoFr's own
// dependency checks stop gating the endpoint, and the datasource sweep behind them is skipped
// entirely, so a probe costs only what the application's checks cost.
//
// Use it when readiness is a relationship between dependencies rather than a conjunction of them —
// "ready if either backing store is up" — or when only some of the registered datasources are
// critical. A check that still wants GoFr's verdict as one input among others can call
// FrameworkReadiness itself.
//
// It is a property of readiness as a whole, not of a single registration: passing it on any one
// AddReadinessCheck call turns framework gating off for the endpoint. GoFr says which mode is in
// force at startup, so this is never inferred from a call site nobody has read.
func ReplaceFrameworkChecks() ReadinessOption {
	return func(r *readinessCheck) { r.replaceFramework = true }
}

// AddReadinessCheck registers a readiness check for /.well-known/health. Without one the endpoint
// behaves exactly as before — HTTP 200 with the aggregate {name, status}, DEGRADED included, since
// which dependencies are critical is an application decision.
//
// By default the check is AND-ed with GoFr's own dependency checks, which are evaluated first:
//
//	app.AddReadinessCheck(func(ctx *gofr.Context) error {
//		return licenseDB.PingContext(ctx)
//	})
//
// Pass ReplaceFrameworkChecks to make the application's checks the whole verdict:
//
//	app.AddReadinessCheck(func(ctx *gofr.Context) error {
//		if redisUp(ctx) || sqlUp(ctx) {
//			return nil
//		}
//
//		return errNoBackingStore
//	}, gofr.ReplaceFrameworkChecks())
//
// Checks accumulate — two independent modules can each register one — and are evaluated in
// registration order, stopping at the first that reports not ready. Register before App.Run.
func (a *App) AddReadinessCheck(check ReadinessCheck, opts ...ReadinessOption) {
	if check == nil {
		return
	}

	r := readinessCheck{check: check}
	for _, opt := range opts {
		opt(&r)
	}

	a.mu.Lock()
	defer a.mu.Unlock()

	a.readinessChecks = append(a.readinessChecks, r)
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
//
// This governs the handler's own log line. The request log written by the logging middleware is
// keyed off the status code alone, so it still reports a probe 503 at ERROR; LOG_DISABLE_PROBES=true
// silences the request log for the probe paths.
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
	a.mu.Lock()
	checks := a.readinessChecks
	a.mu.Unlock()

	// Default behavior, unchanged: with no application check registered the endpoint reports the
	// aggregate dependency status with a 200 — DEGRADED is informational, not a readiness verdict.
	if len(checks) == 0 {
		return healthResponse{Name: c.GetAppName(), Status: aggregateStatus(c)}, nil
	}

	if err := runReadinessChecks(c, checks); err != nil {
		c.Warnf("readiness: not ready: %v", err)

		return nil, errNotReady{}
	}

	return healthResponse{Name: c.GetAppName(), Status: statusUp}, nil
}

// runReadinessChecks evaluates GoFr's own dependency checks — unless they have been replaced — and
// then every registered application check in order, stopping at the first not-ready verdict.
func runReadinessChecks(c *Context, checks []readinessCheck) error {
	if !frameworkReplaced(checks) {
		if err := FrameworkReadiness(c); err != nil {
			return err
		}
	}

	for _, r := range checks {
		if err := r.check(c); err != nil {
			return err
		}
	}

	return nil
}

// frameworkReplaced reports whether any registration disclaimed GoFr's dependency checks.
func frameworkReplaced(checks []readinessCheck) bool {
	for _, r := range checks {
		if r.replaceFramework {
			return true
		}
	}

	return false
}

// FrameworkReadiness renders GoFr's own dependency checks as an error: nil when every registered
// datasource and service is up, and otherwise an error naming the aggregate status — never the
// per-dependency detail behind it, which must not reach the unauthenticated response even by way of
// a check that returns this error unchanged.
//
// A default check never needs it: GoFr evaluates it before the check runs. It is for a check
// registered with ReplaceFrameworkChecks that wants the framework's verdict as one input among
// others rather than as an unconditional gate — tolerating DEGRADED during a warm-up window, say,
// or requiring it only when the application's own dependency is also down. Calling it runs a full
// datasource sweep, so call it once per probe.
//
// It is a function rather than a method on Container because Context embeds *container.Container: a
// method would be promoted onto every handler's ctx, where it would sweep every datasource in the
// hot path.
func FrameworkReadiness(c *Context) error {
	if status := aggregateStatus(c); status != statusUp {
		return fmt.Errorf("%w: reported %s", errFrameworkChecks, status)
	}

	return nil
}

// logReadiness states at startup that readiness is no longer the default, and which of the two
// policies is in force, so a 503 from this endpoint — or a 200 with a dependency down — is never a
// surprise about where the verdict came from.
func (a *App) logReadiness() {
	if a.container.Logger == nil || len(a.readinessChecks) == 0 {
		return
	}

	mode := "enabled, evaluated before the app checks"
	if frameworkReplaced(a.readinessChecks) {
		mode = "replaced by the app checks"
	}

	a.container.Logger.Infof("readiness: %d app check(s) registered - framework checks: %s",
		len(a.readinessChecks), mode)
}

// aggregateStatus runs the full health check and keeps only the overall status, discarding every
// per-dependency detail. The aggregation in Container.appHealth yields "UP" when all dependencies
// are healthy and "DEGRADED" when one or more are down; "DOWN" is a fail-closed default for the
// unreachable case where the aggregate key is missing or not a string.
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
