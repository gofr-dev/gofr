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

### 2. Readiness - /.well-known/health

It is an unauthenticated endpoint that reports whether the service is ready to receive traffic. It
aggregates the health of every registered datasource and service into a single status and returns
**only** the application `name` and that aggregate `status` — `UP` when all dependencies are healthy,
`DEGRADED` when one or more are down.

To avoid leaking infrastructure details on an unauthenticated port, this endpoint intentionally does
**not** expose per-dependency information (hosts, ports, database/keyspace/bucket names, connection
pool stats, usernames, or raw error strings). The full detailed map is still computed internally and
remains available to ops tooling via `Container.Health`.

Sample response when the service is ready (HTTP 200):
```json
{
  "data": {
    "name": "sample-service",
    "status": "UP"
  }
}
```

#### Controlling readiness with `SetHealthCheck`

By default the endpoint always responds with HTTP `200` (readiness probes keep working unchanged).
To hold a pod out of service until its critical dependencies are ready, register a readiness closure.
It receives the request context and the container and returns the status string to report along
with the HTTP status code to respond with. The closure is **not limited to datasources** — it can
combine any checks that define "ready" for your service: datasources on the container, registered
HTTP services, or fully custom logic.

```go
package main

import (
	"context"
	"net/http"

	"gofr.dev/pkg/gofr"
	"gofr.dev/pkg/gofr/container"
)

func main() {
	app := gofr.New()

	// A downstream HTTP dependency this service must be able to reach before it is ready.
	app.AddHTTPService("payment", "https://payment.internal")

	app.SetHealthCheck(func(ctx context.Context, c *container.Container) (status string, statusCode int) {
		// A datasource on the container.
		if c.Redis == nil || c.Redis.Ping(ctx).Err() != nil {
			return "DOWN", http.StatusServiceUnavailable
		}

		// A registered HTTP service — the closure can gate readiness on any dependency,
		// including one reached over HTTP, not just the datasource fields on the container.
		if h := c.GetHTTPService("payment").HealthCheck(ctx); h == nil || h.Status != "UP" {
			return "DOWN", http.StatusServiceUnavailable
		}

		return "UP", http.StatusOK
	})

	app.Run()
}
```

When the closure returns a non-`200` code, `/.well-known/health` responds with that code so
Kubernetes readiness probes stop routing traffic to the pod. Returning `("UP", http.StatusOK)`
(or no closure at all) preserves the default behavior.

> **Note:** The detailed per-dependency health map is being moved to the metrics server
> (`METRICS_PORT`), behind the same network boundary as `/metrics` and `/debug/pprof`. Track that
> work in #3806.

## Related production guides

- **Deploying to Kubernetes**: [Wire `/.well-known/alive` into liveness/readiness probes](/docs/guides/deploying-to-kubernetes) — make Kubernetes act on the health information GoFr exposes.
