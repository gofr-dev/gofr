---
description: "How GoFr handles SIGTERM on Kubernetes: drain HTTP, gRPC, and Pub/Sub, close datasources, and tune terminationGracePeriodSeconds for safe rolling restarts."
nextjs:
  metadata:
    title: "GoFr Graceful Shutdown: SIGTERM, Drain, Kubernetes"
    description: "How GoFr handles SIGTERM on Kubernetes: drain HTTP, gRPC, and Pub/Sub, close datasources, and tune terminationGracePeriodSeconds for safe rolling restarts."
---

# Graceful Shutdown

{% answer %}
GoFr listens for `SIGINT` and `SIGTERM` and, on either signal, runs `App.Shutdown` which calls `Shutdown` on the HTTP, gRPC, and metrics servers and `Close` on the container's datasource connections. The shutdown is bounded by `SHUTDOWN_GRACE_PERIOD` (default `30s`); if it expires the process exits with whatever connections remain. Pair this with Kubernetes' `terminationGracePeriodSeconds` and a small `preStop` sleep to avoid losing in-flight requests during rolling restarts.
{% /answer %}

## When to use

Every production GoFr deployment on Kubernetes should be configured for graceful shutdown. Without it, rolling updates and node drains return 502/504s for any request that is mid-flight when a pod is terminated, and Pub/Sub consumers can lose un-committed messages.

## How GoFr handles signals

`App.Run` sets up a signal-aware context:

```go
ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGINT, syscall.SIGTERM)
```

When that context is canceled, a goroutine creates a timeout context using `SHUTDOWN_GRACE_PERIOD` (default `30s`) and calls `App.Shutdown`. The order is fixed by the framework — see [`pkg/gofr/gofr.go:96-114`](https://github.com/gofr-dev/gofr/blob/main/pkg/gofr/gofr.go) — and `Shutdown` joins errors from each step:

1. `httpServer.Shutdown(ctx)` — stops accepting new connections, waits for in-flight handlers
2. `grpcServer.Shutdown(ctx)` — drains active streams
3. `container.Close()` — closes SQL pools, Redis clients, Pub/Sub consumers, and other registered datasources
4. `metricServer.Shutdown(ctx)` — stops `/metrics`
5. Logger close — if the logger implements `io.Closer`, its `Close()` is called last

The container's `Close` is what commits Pub/Sub offsets and lets SQL drivers finish in-progress queries. Application code does not need to coordinate this order.

## OnStart hooks vs shutdown hooks

GoFr exposes [OnStart hooks](/docs/advanced-guide/startup-hooks) for synchronous startup work (cache warmup, seeding). There is no public `OnShutdown` hook today; `App.Shutdown` is what gets called and it operates on the framework's own resources. If you need cleanup on exit for resources you own (custom goroutines, file handles, third-party clients), use context-cancellation: pass a `context.Context` derived from `signal.NotifyContext(...)` into your goroutines and have each goroutine `defer` its own cleanup when that context is cancelled. The framework's `App.Shutdown` runs concurrently with this, so total wind-down stays within `SHUTDOWN_GRACE_PERIOD`.

## The Kubernetes termination flow

When kubelet decides to evict a pod, it executes this sequence:

1. Pod's status flips to `Terminating`; endpoints controllers begin removing the pod from Service `Endpoints`.
2. `preStop` hook runs (if configured).
3. `SIGTERM` is sent to PID 1.
4. After `terminationGracePeriodSeconds` (default 30s), `SIGKILL` is sent.

Steps 1 and 3 race: kube-proxy on every node needs time to update iptables/IPVS rules. A pod can still receive new traffic for a second or two after `SIGTERM`. The fix is a `preStop` sleep that delays shutdown long enough for endpoint removal to propagate.

```yaml
spec:
  terminationGracePeriodSeconds: 60
  containers:
    - name: api
      image: ghcr.io/example/orders-api:1.4.2
      lifecycle:
        preStop:
          exec:
            command: ["/bin/sh", "-c", "sleep 5"]
      env:
        - name: SHUTDOWN_GRACE_PERIOD
          value: "45s"
      readinessProbe:
        httpGet:
          path: /.well-known/health
          port: 8000
      livenessProbe:
        httpGet:
          path: /.well-known/alive
          port: 8000
```

### Sizing the grace period

Set the values so `preStop` + `SHUTDOWN_GRACE_PERIOD` is comfortably less than `terminationGracePeriodSeconds`. A useful starting point:

- `preStop`: 5s (covers endpoint propagation on most clusters)
- `SHUTDOWN_GRACE_PERIOD`: P99 request latency × 2, plus headroom for Pub/Sub commits
- `terminationGracePeriodSeconds`: `preStop` + `SHUTDOWN_GRACE_PERIOD` + 10s buffer

For a service with 2s P99, that's 5s + 30s + 10s = 45–60s.

## Per-datasource behavior

- **SQL.** `database/sql` waits for active queries to finish on `Close()`. Long-running transactions can extend shutdown — keep request timeouts shorter than `SHUTDOWN_GRACE_PERIOD`.
- **Redis / NoSQL.** Clients close idle connections immediately and wait for in-flight commands.
- **Pub/Sub.** GoFr's subscription manager respects the shutdown context — consumers stop polling and commit current offsets where the broker supports it (Kafka, NATS JetStream).
- **Cron jobs.** GoFr's `App.Shutdown` drains HTTP, gRPC, and metrics servers and closes datasource connections — it does **not** stop the cron scheduler or wait for in-flight cron tasks. Cron jobs run with `context.Background()`, so they continue past SIGTERM and may be cut off when the container is killed at `terminationGracePeriodSeconds`. If you have long-running cron work that must finish, run it as a separate Kubernetes `Job` triggered by a `CronJob` resource instead of inside the same pod, so the pod's lifecycle doesn't interrupt it.

## Verification

Trigger a rolling restart and watch the logs:

```bash
kubectl rollout restart deployment/orders-api -n prod
kubectl logs -f -l app=orders-api -n prod --previous
```

You should see `Shutting down server with a timeout of 30s` followed by `Application shutdown complete` on each terminating pod, with no `connection reset` errors on the client side. From a load-test client running during the restart, error rate should stay below 0.1%.

{% faq %}
{% faq-item question="What is the default SHUTDOWN_GRACE_PERIOD in GoFr?" %}
30 seconds. It is configurable via the `SHUTDOWN_GRACE_PERIOD` env var and accepts any Go duration string (e.g., `45s`, `1m30s`).
{% /faq-item %}
{% faq-item question="Do I need a preStop hook if GoFr already handles SIGTERM?" %}
Yes, on Kubernetes. The preStop sleep covers the brief window before kube-proxy updates iptables on every node — without it, pods can receive new connections after SIGTERM has already started the drain.
{% /faq-item %}
{% faq-item question="What happens if shutdown takes longer than SHUTDOWN_GRACE_PERIOD?" %}
The shutdown context expires, `App.Shutdown` returns the deadline error, and Kubernetes will eventually `SIGKILL` the process when `terminationGracePeriodSeconds` elapses.
{% /faq-item %}
{% /faq %}
