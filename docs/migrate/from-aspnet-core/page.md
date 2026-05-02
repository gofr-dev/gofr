---
description: "Migration guide for C# developers moving from ASP.NET Core to GoFr. Controllers to handlers, DI container to constructors, appsettings.json to .env, OTLP exporter."
nextjs:
  metadata:
    title: "ASP.NET Core to GoFr Migration — Enterprise C# Devs Adopting Go"
    description: "Migration guide for C# developers moving from ASP.NET Core to GoFr. Controllers to handlers, DI container to constructors, appsettings.json to .env, OTLP exporter."
---

# Migrate from ASP.NET Core to GoFr

{% answer %}
ASP.NET Core teams adopting GoFr keep the same operational shape — opinionated framework, built-in DI, configuration, logging, health checks, OpenTelemetry — but lose the class-and-attribute style. Controllers become handler functions, the `IServiceCollection` DI container becomes constructor passing, and `appsettings.json` becomes `.env` files in `configs/`.
{% /answer %}

{% callout title="Migrating with an AI assistant?" %}
Hand [https://gofr.dev/AGENTS.md](https://gofr.dev/AGENTS.md) to your coding assistant (Claude Code, Cursor, Codex, Aider). It contains the framework conventions, routing/binding/datasource patterns, and per-framework cheat-sheets so the assistant can translate handlers without you re-explaining GoFr.
{% /callout %}

## Mental model

| ASP.NET Core | GoFr |
|---|---|
| `[ApiController]` + `[Route("api/users")]` | `app.GET("/api/users", handler)` |
| `[HttpGet("{id}")]` | `app.GET("/api/users/{id}", handler)` |
| `[FromBody] CreateUserDto dto` | `var dto CreateUser; c.Bind(&dto)` |
| `[FromQuery]`, `[FromRoute]` | `c.Param("q")`, `c.PathParam("id")` |
| `IServiceCollection` / `IServiceProvider` | Constructor passing; datasources via `*gofr.Context` |
| `appsettings.json` + environment overlays | `configs/.env` + per-environment files |
| `Configuration.GetSection(...)` | `app.Config.Get(key)` |
| Middleware pipeline (`app.UseX`) | `app.UseMiddleware(...)` |
| `IHostedService` / `BackgroundService` | Goroutines started in `OnStart`, or cron jobs |
| `IHttpClientFactory` + Polly | `app.AddHTTPService` (circuit breaker + retry + rate limit built-in) |
| Health Checks UI | `/.well-known/health` (auto) |
| Serilog / `ILogger<T>` | Built-in structured JSON logger |
| `dotnet ef migrations` | GoFr SQL migrations |
| Hangfire / Quartz | `app.AddCronJob(...)` and Pub/Sub subscribers |

## Side-by-side: controller ↔ handler

**ASP.NET Core:**
```csharp
[ApiController]
[Route("api/users")]
public class UsersController : ControllerBase {
    private readonly IUserService _users;
    public UsersController(IUserService users) => _users = users;

    [HttpPost]
    public async Task<IActionResult> Create([FromBody] CreateUserDto dto) {
        var user = await _users.CreateAsync(dto);
        return Ok(user);
    }
}
```

**GoFr:**
```go
type UsersHandler struct {
    Users UserService
}

func (h *UsersHandler) Create(c *gofr.Context) (any, error) {
    var dto CreateUser
    if err := c.Bind(&dto); err != nil {
        return nil, err
    }
    return h.Users.Create(c, dto)
}

func main() {
    app := gofr.New()
    h := &UsersHandler{Users: NewUserService()}
    app.POST("/api/users", h.Create)
    app.Run()
}
```

## Configuration: appsettings.json → .env

**ASP.NET Core (`appsettings.json`):**
```json
{
  "ConnectionStrings": {
    "Default": "Server=localhost;Database=app;User Id=root"
  },
  "Logging": { "LogLevel": { "Default": "Information" } }
}
```

**GoFr (`configs/.env`):**
```bash
DB_HOST=localhost
DB_NAME=app
DB_USER=root
LOG_LEVEL=INFO
```

Environment-specific files (`configs/.env.production`) layer on top — selected via `APP_ENV`. This is a natural fit for Kubernetes ConfigMaps and Secrets.

## Dependency injection

ASP.NET Core's `IServiceCollection` (transient/scoped/singleton) is replaced by:

- **Constructor passing** — pass dependencies into your handler structs at startup. Sufficient for almost all services.
- **`*gofr.Context`** — datasources (SQL, Redis, Mongo, Pub/Sub clients, HTTP services) are accessed through the request context, so per-request "scoped" services come for free.
- **Wire / Fx** — if you want a generated DI graph, both libraries integrate cleanly.

## Middleware pipeline

ASP.NET Core's `app.UseAuthentication().UseAuthorization()` style maps to:

```go
app.UseMiddleware(authMiddleware)
app.UseMiddleware(rbacMiddleware)
```

Built-in auth options include Basic, API Key, and OAuth/JWT — see [authentication](/docs/advanced-guide/http-authentication). RBAC is supported on top.

## Datasources

`Entity Framework Core`-style ORM is not built in. GoFr provides connection-pooled SQL clients with observability — pair with `sqlc` for type-safe queries if you want EF-like ergonomics. SQL (MySQL/Postgres/Oracle/SQLite/SQL Server), MongoDB, Redis, Cassandra, ScyllaDB, Couchbase, ArangoDB, Dgraph, SurrealDB are supported, with migrations for SQL/Mongo/Redis/Dgraph.

## Observability

OTLP is the lingua franca on both sides — point GoFr at the same collector you already use for `OpenTelemetry.Exporter.OpenTelemetryProtocol`. GoFr emits OpenTelemetry traces, Prometheus metrics at `/metrics`, structured JSON logs with trace IDs, and exposes health at `/.well-known/health`. Log levels can be changed at runtime via the [remote log-level endpoint](/docs/advanced-guide/remote-log-level-change).

## Gradual adoption

Stand up a GoFr microservice next to your ASP.NET Core service. From GoFr, call back into the legacy service through `app.AddHTTPService("legacy", baseURL)` with built-in circuit breaker, retries, and rate limiting. Move endpoints across at the gateway, one bounded context at a time.

{% faq %}

{% faq-item question="Can I run ASP.NET Core and GoFr in the same cluster?" %}
Yes. Both are stateless HTTP/gRPC services. Wire shared OTLP collectors, share auth tokens, and the two interoperate cleanly.
{% /faq-item %}

{% faq-item question="What replaces Entity Framework migrations?" %}
GoFr SQL migrations — versioned, ordered up-migrations applied at boot. See the [migrations guide](/docs/advanced-guide/handling-data-migrations).
{% /faq-item %}

{% faq-item question="What about gRPC services and interceptors?" %}
Supported directly — register your generated `pb` server with GoFr, attach interceptors. See the [gRPC guide](/docs/advanced-guide/grpc).
{% /faq-item %}

{% /faq %}
