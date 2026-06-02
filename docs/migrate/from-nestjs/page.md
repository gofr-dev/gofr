---
description: "Migration guide for TypeScript developers moving from NestJS to GoFr. Controllers and decorators to handlers, modules to constructors, microservices to Pub/Sub."
nextjs:
  metadata:
    title: "NestJS to GoFr Migration — TypeScript Microservices in Go"
    description: "Migration guide for TypeScript developers moving from NestJS to GoFr. Controllers and decorators to handlers, modules to constructors, microservices to Pub/Sub."
---

# Migrate from NestJS to GoFr

{% answer %}
NestJS teams moving to GoFr keep the same architectural shape — controllers, services, validation, microservices — but lose the decorator metaphor. Controllers become plain handler functions, modules become Go packages with explicit constructor wiring, DTO classes become Go structs validated via `c.Bind`, and the `@nestjs/microservices` transports map onto GoFr's built-in Pub/Sub.
{% /answer %}

{% callout title="Migrating with an AI assistant?" %}
Hand [https://gofr.dev/AGENTS.md](https://gofr.dev/AGENTS.md) to your coding assistant (Claude Code, Cursor, Codex, Aider). It contains the framework conventions, routing/binding/datasource patterns, and per-framework cheat-sheets so the assistant can translate handlers without you re-explaining GoFr.
{% /callout %}

## Mental model

NestJS leans on TypeScript decorators and a runtime DI container assembled from `@Module` metadata. Go has neither decorators nor a Nest-style DI container, so the structure becomes more explicit:

| NestJS | GoFr |
|---|---|
| `@Controller('/users')` + `@Get(':id')` | `app.GET("/users/{id}", handler)` |
| `@Body() dto: CreateUserDto` | `var dto CreateUser; c.Bind(&dto)` |
| `@Param('id')` | `c.PathParam("id")` |
| `@Query('q')` | `c.Param("q")` |
| `@Module` + provider injection | Constructor passing of dependencies |
| `Pipes` (validation, transform) | Struct tags + a validator library |
| `Interceptors` / `Guards` | GoFr middleware |
| `@nestjs/microservices` (TCP/Redis/NATS/Kafka) | `app.Subscribe("topic", handler)` over Kafka, NATS, SQS, MQTT, Google Pub/Sub, Azure Event Hub |
| `@nestjs/swagger` | Built-in Swagger UI from your `openapi.json` |
| `@nestjs/typeorm`, `@nestjs/mongoose` | SQL auto-initialized from `DB_DIALECT`/`DB_HOST`/etc. env vars; `app.AddMongo(provider)` for Mongo, plus GoFr migrations |
| `@nestjs/schedule` (`@Cron`) | `app.AddCronJob(...)` |
| `@nestjs/terminus` health | `/.well-known/health` (auto) |

## Side-by-side: controller ↔ handler

**NestJS:**
```ts
@Controller('users')
export class UsersController {
  constructor(private readonly users: UsersService) {}

  @Post()
  async create(@Body() dto: CreateUserDto) {
    return this.users.create(dto);
  }
}
```

**GoFr:**
```go
type UsersHandler struct {
    Users UsersService
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
    h := &UsersHandler{Users: NewUsersService()}
    app.POST("/users", h.Create)
    app.Run()
}
```

## Validation and DTOs

NestJS pairs `class-validator` decorators with a `ValidationPipe`. In GoFr, a DTO is a Go struct with JSON tags; tag-based validation is added by pairing `c.Bind` with `go-playground/validator` (or any validator of your choice).

```go
type CreateUser struct {
    Name  string `json:"name"  validate:"required,min=3"`
    Email string `json:"email" validate:"required,email"`
}
```

## Auto-CRUD via AddRESTHandlers

If you have a typical "Nest CRUD module" — controller + service + entity + repository — GoFr can generate the full CRUD surface for an entity with [`AddRESTHandlers`](/docs/quick-start/add-rest-handlers). One method registers `GET / POST / GET/{id} / PUT/{id} / DELETE/{id}` against your model.

## Microservices and Pub/Sub

`@nestjs/microservices` transports map cleanly:

| Nest transport | GoFr equivalent |
|---|---|
| Kafka | Built-in Kafka subscriber/publisher |
| NATS | Built-in NATS subscriber/publisher |
| Redis Pub/Sub | Use Redis client as datasource |
| RabbitMQ (Nest's `Transport.RMQ`) | Not built into GoFr — use Kafka, NATS, SQS, MQTT, Google Pub/Sub, or Azure Event Hub instead, or bridge via a community driver |
| MQTT | Built-in MQTT subscriber |

Subscribe pattern:
```go
app.Subscribe("user.created", func(c *gofr.Context) error {
    var msg UserCreated
    if err := c.Bind(&msg); err != nil {
        return err
    }
    return process(c, msg)
})
```

## gRPC

For Nest's `@GrpcMethod` setups, GoFr supports gRPC servers and interceptors directly — see the [gRPC guide](/docs/advanced-guide/grpc).

## Configuration

`@nestjs/config` (`.env` + schema) → GoFr loads `configs/.env` (with environment overrides) by default. Read at runtime with `app.Config.Get(key)`.

## Observability

`@nestjs/terminus`, `@willsoto/nestjs-prometheus`, and OpenTelemetry instrumentation are typically wired by hand. GoFr ships OpenTelemetry tracing, Prometheus metrics at `/metrics`, structured JSON logs, `/.well-known/health`, and runtime log-level change.

## Gradual adoption

Stand up a GoFr microservice that owns one bounded context. From the Nest side, call it via HTTP or share a Pub/Sub topic. From GoFr, call back into Nest with `app.AddHTTPService("nest-api", baseURL)` — circuit breaker, retries, and rate limiting are configured per service.

{% faq %}

{% faq-item question="Can I run NestJS and GoFr in the same cluster?" %}
Yes. They are independent processes. Pub/Sub topics and HTTP contracts bridge the two; GoFr's outbound HTTP client adds circuit breaker and retries automatically.
{% /faq-item %}

{% faq-item question="Is there a Nest-style CLI scaffolder?" %}
GoFr provides `AddRESTHandlers` for entity-driven CRUD scaffolding. There isn't a per-resource generator CLI; most teams use editor templates or copy a sample handler.
{% /faq-item %}

{% faq-item question="Do I lose decorator-driven Swagger?" %}
You give up decorator-driven generation, but GoFr serves a built-in Swagger UI from any `openapi.json` you place in the static directory.
{% /faq-item %}

{% /faq %}
