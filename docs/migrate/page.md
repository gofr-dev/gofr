---
description: "Migration guides from Gin, Fiber, Express (Node), Flask (Python), and Spring Boot (Java) to GoFr. Concrete code translations and gradual-adoption strategies."
nextjs:
  metadata:
    title: "Migrate to GoFr — From Gin, Fiber, Express, Flask, Spring Boot"
    description: "Migration guides from Gin, Fiber, Express (Node), Flask (Python), and Spring Boot (Java) to GoFr. Concrete code translations and gradual-adoption strategies."
---

# Migrate to GoFr

{% answer %}
You don't have to migrate everything at once. The recommended path is: pick one new microservice, build it in GoFr, get a feel for the framework, then migrate older services as you touch them. GoFr deploys alongside your existing Gin / Fiber / Echo / Express / Flask / Spring Boot services with no special infrastructure.
{% /answer %}

{% callout title="Migrating with an AI assistant?" %}
Hand [https://gofr.dev/AGENTS.md](https://gofr.dev/AGENTS.md) to your coding assistant (Claude Code, Cursor, Codex, Aider). It contains the framework conventions, routing/binding/datasource patterns, and per-framework cheat-sheets so the assistant can translate handlers without you re-explaining GoFr.
{% /callout %}

## Choose your starting point

### From Go frameworks

- [Migrate from Gin →](/migrate/from-gin) — handler, middleware, binding, and group translations.
- [Migrate from Fiber →](/migrate/from-fiber) — `net/http` semantics, datasource and observability differences.

### From other ecosystems

- [Migrate from Express (Node.js) →](/migrate/from-express) — JavaScript-to-Go mental model, async/await analogues.
- [Migrate from Flask (Python) →](/migrate/from-flask) — Pythonic patterns and their Go equivalents.
- [Migrate from Spring Boot (Java) →](/migrate/from-spring-boot) — DI, controllers, configuration, and observability mappings.

## Recommended adoption strategy

1. **Run a spike.** Build a small new service or internal tool in GoFr to learn the framework patterns.
2. **Establish your baseline configuration.** Decide how your team handles `.env` files, your OpenTelemetry collector endpoint, your Prometheus scrape config, and your log format.
3. **Migrate by attrition.** When you next touch an existing service for a feature or refactor, port it to GoFr in the same change.
4. **Use the same datastores.** GoFr's MySQL / Postgres / Mongo / Redis / Kafka clients connect to the same backends you already use; no data migration is required.
5. **Validate observability.** Confirm that traces, metrics, and logs from the migrated service appear in your existing observability stack with the same names and labels you expect.

## What stays the same

- Your databases, message brokers, and caches. GoFr connects to existing infrastructure.
- Your deployment platform (Kubernetes, ECS, Cloud Run, bare VM — all supported).
- Your CI/CD pipeline. GoFr is a normal Go module; build and ship it the same way.
- Your team's Go skills. GoFr is idiomatic Go.

## What changes

- The handler signature: `func(*gofr.Context) (any, error)` replaces framework-specific types.
- Configuration moves to environment variables / `.env` (12-factor).
- Observability becomes default — you remove your manual OpenTelemetry / Prometheus wiring code.
- Datasource access goes through `c.SQL`, `c.Redis`, `c.Mongo`, etc., instead of injected clients you manage.

{% faq %}

{% faq-item question="Can I migrate one route at a time?" %}
Within a service: not easily, since GoFr owns the HTTP server. Across services: yes — keep your existing services running and migrate them one at a time.
{% /faq-item %}

{% faq-item question="Does my existing OpenTelemetry collector / Prometheus / log aggregator still work?" %}
Yes. GoFr exports OTLP traces and Prometheus metrics; structured logs go to stdout in JSON.
{% /faq-item %}

{% /faq %}
