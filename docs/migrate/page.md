---
description: "Migration guides to GoFr from Gin, Fiber, Echo, chi, Express, NestJS, Flask, FastAPI, Django REST, Spring Boot, ASP.NET Core, Laravel, and Rails. Concrete code translations and gradual-adoption strategies."
nextjs:
  metadata:
    title: "Migrate to GoFr — From Gin, Fiber, Echo, chi, Express, NestJS, Flask, FastAPI, Django REST, Spring Boot, ASP.NET Core, Laravel, Rails"
    description: "Migration guides to GoFr from Gin, Fiber, Echo, chi, Express, NestJS, Flask, FastAPI, Django REST, Spring Boot, ASP.NET Core, Laravel, and Rails. Concrete code translations and gradual-adoption strategies."
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
- [Migrate from Echo →](/migrate/from-echo) — Migration guide for Go developers moving from Echo to GoFr. Handler signature, middleware, route groups, binding, and gradual adoption with side-by-side examples.
- [Migrate from chi →](/migrate/from-chi) — Migration guide for Go developers moving from chi router to GoFr framework. Handler signature, middleware, route groups, URL params, and the router-vs-framework trade-off.

### From Node.js / TypeScript

- [Migrate from Express (Node.js) →](/migrate/from-express) — JavaScript-to-Go mental model, async/await analogues.
- [Migrate from NestJS →](/migrate/from-nestjs) — Migration guide for TypeScript developers moving from NestJS to GoFr. Controllers and decorators to handlers, modules to constructors, microservices to Pub/Sub.

### From Python

- [Migrate from Flask →](/migrate/from-flask) — Pythonic patterns and their Go equivalents.
- [Migrate from FastAPI →](/migrate/from-fastapi) — Migration guide for Python developers moving from FastAPI to GoFr. Async/await to goroutines, Pydantic to Go structs, automatic OpenAPI to built-in Swagger UI.
- [Migrate from Django REST →](/migrate/from-django-rest) — Migration guide for Python developers moving from Django REST Framework to GoFr. ViewSets to AddRESTHandlers, ORM to SQL drivers, permissions to RBAC, settings.py to .env.

### From Java / .NET

- [Migrate from Spring Boot (Java) →](/migrate/from-spring-boot) — DI, controllers, configuration, and observability mappings.
- [Migrate from ASP.NET Core →](/migrate/from-aspnet-core) — Migration guide for C# developers moving from ASP.NET Core to GoFr. Controllers to handlers, DI container to constructors, appsettings.json to .env, OTLP exporter.

### From PHP / Ruby

- [Migrate from Laravel (PHP) →](/migrate/from-laravel) — Migration guide for PHP developers moving from Laravel to GoFr. Controllers to handlers, Eloquent to SQL drivers, Artisan to GoFr CLI, queues to Pub/Sub.
- [Migrate from Rails (Ruby) →](/migrate/from-rails) — Migration guide for Ruby developers moving from Rails to GoFr. Controllers to handlers, ActiveRecord to SQL, Active Job to Pub/Sub, Action Cable to WebSocket.

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
