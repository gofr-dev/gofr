---
description: "Health checks in GoFr probe each service's status and dependencies, helping detect failures, prevent cascades, and route traffic in distributed systems."
nextjs:
  metadata:
    title: "Monitoring Service Health in GoFr — Health Checks"
    description: "Health checks in GoFr probe each service's status and dependencies, helping detect failures, prevent cascades, and route traffic in distributed systems."
---

# Monitoring Service Health

Health check in microservices refers to a mechanism or process implemented within each service to assess its operational status
and readiness to handle requests. It involves regularly querying the service to determine if it is functioning correctly,
typically by evaluating its responsiveness and ability to perform essential tasks. Health checks play a critical role in ensuring service availability,
detecting failures, preventing cascading issues, and facilitating effective traffic routing in distributed systems.

## GoFr by default registers two endpoints which are:

### 1. Aliveness - /.well-known/alive

It is an endpoint which returns the following response with a 200 status code, when the service is UP.

```json
{
  "data": {
    "status": "UP"
  }
}
```

It is also used when state of {% new-tab-link newtab=false title="circuit breaker" href="/docs/advanced-guide/circuit-breaker" /%} is open.

To override this endpoint, pass the following option while registering HTTP Service:
```go
&service.HealthConfig{
			HealthEndpoint: "breeds",
		}
```

### 2. Health-Check - /.well-known/health

It is an unauthenticated endpoint that reports an **aggregate status** for the application. It
aggregates the health of every registered datasource and service into a single status and returns
**only** the application `name` and that aggregate `status` — `UP` when all dependencies are healthy,
`DEGRADED` when one or more are down.

Note that the endpoint answers `200` in both cases: a `DEGRADED` aggregate does **not** produce a
non-2xx response, so a probe wired directly to the status code will always see the service as ready.
A caller that wants to act on degradation must read the `status` field itself, or register a
[readiness check](#controlling-readiness-with-a-readiness-check) so the endpoint answers `503` when
the service is not ready to serve traffic.

To avoid leaking infrastructure details on an unauthenticated port, this endpoint intentionally does
**not** expose per-dependency information (hosts, ports, database/keyspace/bucket names, connection
pool stats, usernames, or raw error strings). No HTTP route serves the detailed map after this
change — `Container.Health` still computes it for in-process ops tooling.

> **Changed response shape:** this endpoint previously returned the full per-dependency map. It now
> returns `{name, status}` only. The HTTP status code is unchanged (`200`), so existing readiness
> probes keep working; anything parsing the body for dependency details must be updated. The
> framework `version` field is also gone — it is exactly what an attacker enumerating known CVEs
> wants from an unauthenticated endpoint, so dropping it is part of the fix, but it is also the
> field most likely to be on an existing dashboard.

Sample response when the service is ready (HTTP 200):
```json
{
  "data": {
    "name": "sample-service",
    "status": "UP"
  }
}
```

#### Controlling readiness with a readiness check

By default the endpoint always responds with HTTP `200` (existing probes keep working unchanged).
To hold a pod out of service until it can actually serve traffic, register a readiness check:

```go
type ReadinessCheck func(ctx *gofr.Context) error

func (a *App) AddReadinessCheck(check ReadinessCheck, opts ...ReadinessOption)
func ReplaceFrameworkChecks() ReadinessOption
```

Return `nil` when ready, any error when not — in which case the endpoint answers `503` and
Kubernetes stops routing traffic to the pod. The error is logged, never written to the response:
the endpoint is unauthenticated and must not leak dependency details.

Checks accumulate, so two independent modules can each register one; they are evaluated in
registration order and the first not-ready verdict wins.

##### Requiring GoFr's checks in addition to your own

This is the default: GoFr's own verdict on every registered datasource and service is evaluated
first, and your check runs only if it passes. Because the check is a closure it can probe
dependencies GoFr never registered — a datasource you opened yourself, a third-party SDK client, an
in-process warm-up flag:

```go
package main

import (
	"database/sql"
	"errors"
	"sync/atomic"

	_ "github.com/lib/pq" // driver for the app-managed datasource below

	"gofr.dev/pkg/gofr"
)

// cacheWarm is flipped once the in-process cache has been populated.
var cacheWarm atomic.Bool

var errCacheNotWarm = errors.New("cache not warm")

func main() {
	app := gofr.New()

	// A downstream HTTP dependency — already covered by GoFr's own checks, which gate readiness
	// before the check below is called.
	app.AddHTTPService("payment", "https://payment.internal")

	// An external datasource the app connects to itself: GoFr's container has no handle on it,
	// so readiness can only account for it through a check that captures it.
	licenseDB := openLicenseStore(app)

	app.AddReadinessCheck(func(ctx *gofr.Context) error {
		if err := licenseDB.PingContext(ctx); err != nil {
			return err
		}

		if !cacheWarm.Load() {
			return errCacheNotWarm
		}

		return nil
	})

	app.Run()
}

// openLicenseStore connects to a datasource the application manages directly, outside GoFr.
func openLicenseStore(app *gofr.App) *sql.DB {
	db, err := sql.Open("postgres", app.Config.Get("LICENSE_DB_DSN"))
	if err != nil {
		app.Logger().Fatalf("license store: %v", err)
	}

	return db
}
```

##### Replacing GoFr's verdict

Pass `gofr.ReplaceFrameworkChecks()` and readiness is exactly what your checks say. This is what
readiness that is a *relationship between* dependencies needs, which a conjunction of independently
evaluated checks cannot express. For example, when the service can serve from either backing store:

```go
app.AddReadinessCheck(func(ctx *gofr.Context) error {
	if redisUp(ctx) || sqlUp(ctx) {
		return nil
	}

	return errNoBackingStore
}, gofr.ReplaceFrameworkChecks())
```

The same shape narrows readiness when only one of several registered datasources is critical:

```go
app.AddReadinessCheck(func(ctx *gofr.Context) error {
	if h := ctx.SQL.HealthCheck(); h == nil || h.Status != "UP" {
		return errors.New("postgres not reachable")
	}

	return nil
}, gofr.ReplaceFrameworkChecks())
```

The option is a property of readiness as a whole, not of one registration: passing it on any single
`AddReadinessCheck` call turns framework gating off for the endpoint, and GoFr says which mode is in
force at startup rather than leaving it to be inferred from a call site.

Because GoFr then knows its own checks no longer decide anything, it **skips the datasource sweep
entirely** — a probe costs only what your checks cost. That matters when a dependency is slow or
hanging, which is exactly when the sweep is expensive and the readiness probe earns its keep:
`readinessProbe.timeoutSeconds` is small (the
[Kubernetes guide](/docs/guides/deploying-to-kubernetes) ships `2`), so a check that disclaims those
dependencies should not be timed out by them.

A check that wants GoFr's verdict as one input among others — tolerating `DEGRADED` during a warm-up
window, or requiring it only when its own dependency is also down — can ask for it explicitly:

```go
app.AddReadinessCheck(func(ctx *gofr.Context) error {
	if err := gofr.FrameworkReadiness(ctx); err != nil && time.Since(started) > warmup {
		return err
	}

	return nil
}, gofr.ReplaceFrameworkChecks())
```

`gofr.FrameworkReadiness` returns `nil` when every registered datasource and service is up, and
otherwise an error naming the aggregate status only — never the per-dependency detail behind it, so
a check that returns it unchanged still cannot leak anything to the unauthenticated response. It
runs a full datasource sweep, so call it at most once per probe.

##### What you see at startup and at runtime

When a check is registered, GoFr says so at startup — including which of the two policies is in
force, so a `503` from this endpoint, or a `200` with a dependency down, is never a surprise about
where the verdict came from:

```
INFO  readiness: 1 app check(s) registered - framework checks: enabled, evaluated before the app checks
INFO  readiness: 2 app check(s) registered - framework checks: replaced by the app checks
```

A not-ready result responds `503` with `{"error":{"message":"DOWN"}}`. The reason — the error your
check returned — is logged at `WARN`, not `ERROR`: a `503` here is expected during startup and
rolling deploys, and Kubernetes probes it repeatedly.

The **request log** is a separate line and is keyed off the status code alone, so it reports every
probe `503` at `ERROR`. During a rolling deploy that is a burst of `ERROR`-level lines for an
expected condition, which a level-keyed alert will pick up. Set `LOG_DISABLE_PROBES=true` to silence
the request log for `/.well-known/health` and `/.well-known/alive`; the `WARN` line carrying the
reason is unaffected.

##### Readiness does not propagate to GoFr callers

`AddReadinessCheck` gates the load balancer, and nothing else. When another GoFr application depends
on this one through `AddHTTPService`, its dependency check probes `/.well-known/alive`, not
`/.well-known/health` — so a caller reports this service `UP` even while its readiness check is
answering `503`.

That is deliberate: `/.well-known/alive` answers immediately, whereas `/.well-known/health` runs the
callee's own dependency sweep, which would make every caller re-run its downstream's sweep and
report as down any downstream slower than the health-check timeout — while it is serving traffic
normally. Readiness is a signal for the orchestrator routing traffic to this pod, not for
application-level dependency checks.

> **Note:** The detailed per-dependency health map is being moved to the metrics server
> (`METRICS_PORT`), behind the same network boundary as `/metrics` and `/debug/pprof`. Track that
> work in #3806.

## Related production guides

- **Deploying to Kubernetes**: [Wire `/.well-known/alive` into liveness/readiness probes](/docs/guides/deploying-to-kubernetes) — make Kubernetes act on the health information GoFr exposes.
