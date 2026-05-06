---
description: "Migration guide for Java developers moving from Spring Boot to GoFr. Controller-to-handler translation, dependency injection, configuration, and observability mapping."
nextjs:
  metadata:
    title: "Spring Boot (Java) to GoFr Migration — Java Devs Adopting Go"
    description: "Migration guide for Java developers moving from Spring Boot to GoFr. Controller-to-handler translation, dependency injection, configuration, and observability mapping."
---

# Migrate from Spring Boot (Java) to GoFr

{% answer %}
Spring Boot developers tend to feel at home in GoFr — both frameworks share an opinionated, batteries-included philosophy. Spring's controllers map to GoFr handlers; Spring's auto-configuration maps to GoFr's defaults; Spring's Actuator endpoints map to GoFr's built-in `/.well-known/health` and `/metrics`.
{% /answer %}

{% callout title="Migrating with an AI assistant?" %}
Hand [https://gofr.dev/AGENTS.md](https://gofr.dev/AGENTS.md) to your coding assistant (Claude Code, Cursor, Codex, Aider). It contains the framework conventions, routing/binding/datasource patterns, and per-framework cheat-sheets so the assistant can translate handlers without you re-explaining GoFr.
{% /callout %}

## Mental model translation

| Spring Boot | GoFr |
|---|---|
| `@RestController` + `@RequestMapping` | `app.GET("/path", handler)` |
| `@PathVariable` | `c.PathParam("id")` |
| `@RequestParam` | `c.Param("q")` |
| `@RequestBody` | `c.Bind(&struct)` |
| `@Autowired` field injection | Struct fields populated via constructor or via GoFr's container |
| `application.yaml` | `.env` + GoFr Configs |
| `Spring Data JPA` | Plain SQL via `c.SQL` (or `sqlc` / `gorm` if you want ORM-like) |
| `@Scheduled` cron | `app.AddCronJob(...)` |
| `@KafkaListener` | `app.Subscribe("topic", handler)` |
| Spring Actuator | Built-in `/.well-known/health`, `/metrics` |
| Micrometer | Built-in Prometheus metrics |
| Spring Cloud Sleuth | Built-in OpenTelemetry tracing |
| Resilience4j circuit breaker | Built-in via `app.AddHTTPService` |

## Hello world

**Spring Boot:**
```java
@RestController
class HelloController {
    @GetMapping("/hello")
    public Map<String, String> hello() {
        return Map.of("message", "Hello, world");
    }
}
```

**GoFr:**
```go
package main

import "gofr.dev/pkg/gofr"

func main() {
    app := gofr.New()
    app.GET("/hello", func(c *gofr.Context) (any, error) {
        return "Hello, world", nil
    })
    app.Run()
}
```

## Configuration: application.yaml → .env

**Spring Boot:**
```yaml
spring:
  datasource:
    url: jdbc:mysql://localhost:3306/mydb
    username: root
server:
  port: 8080
```

**GoFr (`configs/.env`):**
```bash
HTTP_PORT=8080
DB_HOST=localhost
DB_PORT=3306
DB_NAME=mydb
DB_USER=root
```

12-factor / environment-variable configuration is GoFr's default; per-environment files are a natural fit for Kubernetes ConfigMaps and Secrets.

## Dependency injection

Go does not have class-based DI like Spring. The conventions are:

- **Plain struct composition.** Most services pass dependencies through constructors. This is enough for the majority of cases.
- **GoFr Container.** Datasources (SQL, Redis, Mongo, Pub/Sub clients) are provided by GoFr's Container and accessed via the request `Context`.
- **Wire or Fx.** If you want a generated DI graph, both libraries integrate cleanly with GoFr.

## What you can drop

- Spring Web Starter, Spring Data, Spring Security, Spring Cloud — replaced by GoFr's bundled equivalents.
- Spring Boot Actuator, Micrometer, Sleuth — replaced by GoFr's built-in observability.
- Resilience4j patterns on outbound HTTP — built into GoFr's service-to-service client.

## What you'll likely appreciate

- **Startup time** measured in tens of milliseconds, not seconds.
- **Memory footprint** of tens of MB, not hundreds.
- **No JVM tuning, no GC pauses to chase.** Go's runtime is forgiving.
- **Single static binary deploy.** Smaller container images.

## Common gotchas

- **No annotation-driven anything.** Routing, validation, security, transactions — all explicit code, not annotations on classes.
- **No JPA-style lazy loading.** SQL is explicit. If you depend on lazy-loaded relations, plan to do JOINs or eager-load explicitly.
- **No `application-prod.yaml` / `application-staging.yaml` profile system.** GoFr loads `configs/.env` and then overlays `configs/.<APP_ENV>.env` on top — so `APP_ENV=production` reads `configs/.env` then `configs/.production.env`. Note the dot prefix and `.env` suffix on the override file (not `.env.production`).
- **No bean lifecycle.** Replace `@PostConstruct` / `@PreDestroy` with `OnStart` and graceful shutdown in `main`.
- **Generics syntax is different from Java.** Go generics exist but are used sparingly; most code reads more like pre-generics Java.
- **Dependency injection is wiring, not magic.** `@Autowired` field injection becomes constructor parameters, or `Wire` / `Fx` if you want a generated graph. See [Spring DI patterns and their Go equivalents](/docs/references/context).

## Estimated effort per service

A medium Spring Boot service (50-100 endpoints, JPA entities, Kafka listeners) typically takes 1–2 engineering weeks for a Java team. The biggest time sink is decomposing JPA-heavy data access patterns into explicit Go SQL.

## Recommended adoption

1. Pick a Spring Boot service that's stateless and well-tested.
2. Rebuild it in GoFr; reuse its database, message broker, and downstream contracts.
3. Compare resource consumption and latency on identical workloads.

{% faq %}

{% faq-item question="What about Spring's @Transactional?" %}
Go has explicit transaction handling — `tx, _ := c.SQL.Begin(); defer tx.Rollback(); ...; tx.Commit()`. There is no annotation-driven transaction boundary; the boundary is wherever your code says it is.
{% /faq-item %}

{% faq-item question="Can I keep using Kafka with the same topics?" %}
Yes. GoFr's Pub/Sub subscriber connects to your existing Kafka brokers and topics.
{% /faq-item %}

{% /faq %}
