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
A caller that wants to act on degradation must read the `status` field itself.

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

> **Note:** The detailed per-dependency health map is being moved to the metrics server
> (`METRICS_PORT`), behind the same network boundary as `/metrics` and `/debug/pprof`. Track that
> work in #3806.

## Related production guides

- **Deploying to Kubernetes**: [Wire `/.well-known/alive` into liveness/readiness probes](/docs/guides/deploying-to-kubernetes) — make Kubernetes act on the health information GoFr exposes.
