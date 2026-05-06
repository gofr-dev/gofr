---
description: "Learn GoFr through three curated tracks: coming from another language, Go developer new to GoFr, or building for production. Each track is a sequenced reading list."
nextjs:
  metadata:
    title: "Learn GoFr — Structured Tracks for Every Background"
    description: "Learn GoFr through three curated tracks: coming from another language, Go developer new to GoFr, or building for production. Each track is a sequenced reading list."
---

# Learn GoFr

{% answer %}
There's no single right way to learn a framework — the best path depends on where you're starting from. Three tracks below sequence the existing GoFr documentation by background and goal: coming from another language, experienced Go developer new to GoFr, or building for production.
{% /answer %}

## Track A — Coming from another language

**Estimated time: 1–2 hours (read) + 30 min (run hello-world).**

You know how to build microservices in Node, Python, Java, or another ecosystem, but you're new to Go. Read in this order:

1. [Quick Start: Build your first GoFr REST API](/docs/quick-start/introduction) — get something running.
2. **Pick your migration guide:**
   - [From Express (Node.js)](/migrate/from-express)
   - [From Flask (Python)](/migrate/from-flask)
   - [From Spring Boot (Java)](/migrate/from-spring-boot)
3. [Configuration](/docs/quick-start/configuration) — environment-driven config.
4. [Connecting MySQL](/docs/quick-start/connecting-mysql) and [Connecting Redis](/docs/quick-start/connecting-redis).
5. [Observability](/docs/quick-start/observability) — see traces, metrics, and structured logs.
6. [GoFr Context Reference](/docs/references/context) — the one core abstraction.

## Track B — Go developer new to GoFr

**Estimated time: 30 min (read) + 30 min (run hello-world).**

You've written Go before — maybe with `net/http`, Gin, Fiber, Echo, or Chi.

1. [Why GoFr?](/why-gofr) — the philosophy and what's actually in the box.
2. [GoFr vs your current framework](/comparison) — head-to-head feature comparison.
3. [Quick Start: Build your first GoFr REST API](/docs/quick-start/introduction).
4. [Auto CRUD REST handlers](/docs/quick-start/add-rest-handlers).
5. [Custom middleware](/docs/advanced-guide/middlewares).
6. [Service-to-service HTTP](/docs/advanced-guide/http-communication).
7. [GoFr Context Reference](/docs/references/context) and [Configuration Reference](/docs/references/configs).

## Track C — Building for production

**Estimated time: 2–3 hours, dipping in as you encounter each concern in real services.**

1. [Observability](/docs/quick-start/observability).
2. [Custom OpenTelemetry spans](/docs/advanced-guide/custom-spans-in-tracing).
3. [Custom Prometheus metrics](/docs/advanced-guide/publishing-custom-metrics).
4. [Service health monitoring](/docs/advanced-guide/monitoring-service-health).
5. [Authentication](/docs/advanced-guide/authentication) and [RBAC](/docs/advanced-guide/rbac).
6. [Circuit breaker support](/docs/advanced-guide/circuit-breaker) and [HTTP communication](/docs/advanced-guide/http-communication).
7. [Database migrations](/docs/advanced-guide/handling-data-migrations).
8. [Startup hooks](/docs/advanced-guide/startup-hooks).
9. [Remote log level change](/docs/advanced-guide/remote-log-level-change).
10. [Profiling (pprof)](/docs/advanced-guide/debugging).
11. [Testing](/docs/references/testing).

## Reference materials

- [Full documentation index](/docs)
- [Examples repository](https://github.com/gofr-dev/gofr/tree/main/examples)
- [Showcase](/showcase) — companies and engineers running GoFr in production.
- [Changelog](/changelog) — release notes and version history.
