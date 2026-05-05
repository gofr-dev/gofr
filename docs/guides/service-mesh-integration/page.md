---
description: "Run GoFr services on Istio or Linkerd: when a mesh helps versus GoFr's built-in resilience, mTLS, tracing handoff, and avoiding double retries."
nextjs:
  metadata:
    title: "GoFr Service Mesh Integration: Istio & Linkerd Guide"
    description: "Run GoFr services on Istio or Linkerd: when a mesh helps versus GoFr's built-in resilience, mTLS, tracing handoff, and avoiding double retries."
---

# Service Mesh Integration

{% answer %}
GoFr services run unchanged on Istio or Linkerd because the framework speaks plain HTTP/gRPC. The mesh adds mTLS, traffic policy, and L7 telemetry through a sidecar — but you should pick one owner for retries and circuit breaking, since GoFr's HTTP client already provides both.
{% /answer %}

## When a mesh helps versus library-level resilience

GoFr already ships several patterns commonly cited as reasons to adopt a mesh:

- Service-to-service HTTP client with [Circuit Breaker](/docs/advanced-guide/circuit-breaker), retry, and rate-limit options on `AddHTTPService`.
- W3C TraceContext propagation for outbound calls (verified in `pkg/gofr/service/new.go`).
- Health endpoints `/.well-known/health` and `/.well-known/alive` for readiness/liveness probes.

A mesh becomes worth its sidecar overhead when you need:

- mTLS between every pod without code changes.
- Traffic shifting / canary by percentage at L7.
- Mesh-wide policy (deny-all by default, then allowlist).
- A consistent telemetry plane across services written in different languages.

If your fleet is GoFr-only and you mainly want resilience, GoFr's built-in features may be enough.

## mTLS without code changes

In Istio, apply a `PeerAuthentication` policy in `STRICT` mode and a `DestinationRule` with `tls.mode: ISTIO_MUTUAL`. GoFr requires no change — the sidecar transparently terminates and re-encrypts traffic.

In Linkerd, mTLS is automatic between meshed pods. Annotate the namespace with `linkerd.io/inject: enabled` and redeploy.

For the exact CRD syntax, follow the canonical docs:
- Istio: `https://istio.io/latest/docs/tasks/security/authentication/`
- Linkerd: `https://linkerd.io/2/features/automatic-mtls/`

## Tracing: mesh spans on top of GoFr's

GoFr emits W3C TraceContext (`traceparent`, `tracestate`) on inbound requests and propagates them on outbound HTTP service calls. When you add a mesh:

- Istio injects its own server/client spans wrapping GoFr's spans, giving you network-layer timing alongside your application spans.
- Both stacks must agree on the propagator. GoFr uses `propagation.TraceContext` + `Baggage` (see `pkg/gofr/otel.go`), which matches the W3C standard Istio and Linkerd use.
- Configure your mesh's tracer to send to the same backend (Jaeger, Tempo, OTLP collector) you point GoFr at via `TRACE_EXPORTER` and `TRACER_URL`.

## Retries and circuit breaker: pick one owner

This is where teams burn themselves. If both GoFr and the mesh retry, a 503 on a downstream service can multiply into 9+ retries (3 from GoFr times 3 from the mesh).

Recommendation: **own resilience in one layer, not both.**

- If you want consistent behavior across HTTP and gRPC and you are already using GoFr's `AddHTTPService` with `CircuitBreakerConfig` and `RetryConfig`: turn off mesh-level retries and outlier detection for those routes.
- If you want polyglot uniformity: rely on the mesh, and call `AddHTTPService` without retry/circuit-breaker options.

GoFr's circuit breaker uses `/.well-known/alive` to probe recovery. If you delegate to the mesh, the mesh's outlier detection plays the same role.

## Sidecar overhead

A sidecar adds CPU, memory, and ~1–3ms of latency per hop. For a low-QPS internal service the overhead is usually fine; for a hot path with strict latency budgets, benchmark before adopting. GoFr's library-level resilience has no sidecar cost.

## Probes still go to GoFr

Set Kubernetes probes on the GoFr ports, not the sidecar:

```yaml
livenessProbe:
  httpGet:
    path: /.well-known/alive
    port: 8000
readinessProbe:
  httpGet:
    path: /.well-known/health
    port: 8000
```

`/.well-known/alive` is the liveness signal; `/.well-known/health` includes dependency status and may be slower.

{% faq %}
{% faq-item question="Do I need to change GoFr code to enable mTLS via Istio or Linkerd?" %}
No. The sidecar handles TLS at the network layer, so a plain HTTP listener inside the pod is fine. You only change Kubernetes manifests.
{% /faq-item %}
{% faq-item question="Should the mesh or GoFr own retries?" %}
Pick one. Running both layers at default settings can multiply request volume on a struggling downstream. If you keep GoFr's `RetryConfig`, disable mesh retries for those routes.
{% /faq-item %}
{% faq-item question="Will mesh-injected spans break GoFr tracing?" %}
No. GoFr uses W3C TraceContext, the same standard Istio and Linkerd use, so spans stitch together if both export to the same collector.
{% /faq-item %}
{% /faq %}
