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
func (a *App) SetReadinessCheck(check func(ctx *gofr.Context, framework error) error)
```

Return `nil` when ready, any error when not — in which case the endpoint answers `503` and
Kubernetes stops routing traffic to the pod. The error is logged, never written to the response:
the endpoint is unauthenticated and must not leak dependency details.

`framework` is GoFr's own verdict on every registered datasource and service: `nil` when they are
all up, non-nil when one or more are not. What the check does with it *is* the readiness policy —
so the same single method covers both composing with GoFr's checks and replacing them.

##### Requiring GoFr's checks in addition to your own

Propagate `framework` and the endpoint gates on both. Because the check is a closure it can also
probe dependencies GoFr never registered — a datasource you opened yourself, a third-party SDK
client, an in-process warm-up flag:

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

	// A downstream HTTP dependency — already covered by GoFr's own checks.
	app.AddHTTPService("payment", "https://payment.internal")

	// An external datasource the app connects to itself: GoFr's container has no handle on it,
	// so readiness can only account for it through a check that captures it.
	licenseDB := openLicenseStore(app)

	app.SetReadinessCheck(func(ctx *gofr.Context, framework error) error {
		// Every registered datasource and service must be up ...
		if framework != nil {
			return framework
		}

		// ... and so must the dependencies GoFr does not know about.
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

Ignore `framework` — writing `_`, so the override is visible at the call site — and readiness is
exactly what your check says. This is what readiness that is a *relationship between* dependencies
needs, which a set of independently-evaluated checks could not express. For example, when the
service can serve from either backing store:

```go
app.SetReadinessCheck(func(ctx *gofr.Context, _ error) error {
	if redisUp(ctx) || sqlUp(ctx) {
		return nil
	}

	return errNoBackingStore
})
```

The same shape narrows readiness when only one of several registered datasources is critical:

```go
app.SetReadinessCheck(func(ctx *gofr.Context, _ error) error {
	if h := ctx.SQL.HealthCheck(); h == nil || h.Status != "UP" {
		return errors.New("postgres not reachable")
	}

	return nil
})
```

Note that GoFr's checks still *run* — `framework` is computed before your check is called — so a
check that ignores the parameter pays for the datasource sweep without using its result. That
matches the default path, which sweeps on every probe too.

##### What you see at startup and at runtime

When a check is registered, GoFr says so at startup, so a `503` from this endpoint is never a
surprise about where the verdict came from:

```
INFO  readiness: app check registered - framework checks: enabled, propagated to the app check
```

A not-ready result responds `503` with `{"error":{"message":"DOWN"}}`. The reason — the error your
check returned — is logged at `WARN`, not `ERROR`: a `503` here is expected during startup and
rolling deploys, and Kubernetes probes it repeatedly.

> **Note:** The detailed per-dependency health map is being moved to the metrics server
> (`METRICS_PORT`), behind the same network boundary as `/metrics` and `/debug/pprof`. Track that
> work in #3806.

## Related production guides

- **Deploying to Kubernetes**: [Wire `/.well-known/alive` into liveness/readiness probes](/docs/guides/deploying-to-kubernetes) — make Kubernetes act on the health information GoFr exposes.
