---
description: "Migration guide for Python developers moving from Django REST Framework to GoFr. ViewSets to AddRESTHandlers, ORM to SQL drivers, permissions to RBAC, settings.py to .env."
nextjs:
  metadata:
    title: "Django REST to GoFr Migration — Python REST Devs Adopting Go"
    description: "Migration guide for Python developers moving from Django REST Framework to GoFr. ViewSets to AddRESTHandlers, ORM to SQL drivers, permissions to RBAC, settings.py to .env."
---

# Migrate from Django REST Framework to GoFr

{% answer %}
Django REST Framework's `ModelViewSet` + `ModelSerializer` pattern maps onto GoFr's `AddRESTHandlers`, which generates the standard CRUD surface against a Go struct. The Django ORM is replaced by GoFr's SQL clients (with explicit queries — no ORM); DRF permissions become GoFr RBAC and middleware; `settings.py` becomes `.env` in `configs/`.
{% /answer %}

{% callout title="Migrating with an AI assistant?" %}
Hand [https://gofr.dev/AGENTS.md](https://gofr.dev/AGENTS.md) to your coding assistant (Claude Code, Cursor, Codex, Aider). It contains the framework conventions, routing/binding/datasource patterns, and per-framework cheat-sheets so the assistant can translate handlers without you re-explaining GoFr.
{% /callout %}

## Mental model

| Django REST | GoFr |
|---|---|
| `ModelViewSet` | `app.AddRESTHandlers(&Entity{})` (auto CRUD) |
| `APIView` | `app.GET/POST/...` with handler functions |
| `ModelSerializer` | Go struct + JSON tags |
| Validators on serializer fields | Struct validation tags + a validator library |
| `request.data` | `c.Bind(&dto)` |
| URL routers / `router.register` | `app.GET/POST(...)` per route |
| `IsAuthenticated`, custom permissions | GoFr Basic / APIKey / OAuth-JWT auth + RBAC |
| `settings.py` | `configs/.env` |
| Django signals | No direct equivalent — use Pub/Sub for cross-service events |
| `manage.py migrate` | GoFr SQL migrations |
| Celery | `app.AddCronJob(...)` and Pub/Sub subscribers |
| `django-prometheus` / `OpenTelemetry` | Built into GoFr |

## Side-by-side: ViewSet ↔ AddRESTHandlers

**Django REST:**
```python
class UserViewSet(viewsets.ModelViewSet):
    queryset = User.objects.all()
    serializer_class = UserSerializer

router = DefaultRouter()
router.register('users', UserViewSet)
```

**GoFr:**
```go
type User struct {
    ID    int    `json:"id"`
    Name  string `json:"name"`
    Email string `json:"email"`
}

func main() {
    app := gofr.New()
    if err := app.AddRESTHandlers(&User{}); err != nil { // GET, POST, GET/{id}, PUT/{id}, DELETE/{id}
        app.Logger().Fatal(err)
    }
    app.Run()
}
```

`AddRESTHandlers` reads the struct, infers the table, and exposes the five standard CRUD endpoints. For anything custom, fall back to plain `app.GET/POST/...` handlers. See the [REST scaffolding guide](/docs/quick-start/add-rest-handlers).

## Custom views

For non-CRUD logic, write a handler:

```go
app.POST("/users/{id}/reset-password", func(c *gofr.Context) (any, error) {
    id := c.PathParam("id")
    var dto ResetPassword
    if err := c.Bind(&dto); err != nil {
        return nil, err
    }
    return resetPassword(c, id, dto)
})
```

## Serializers and validation

DRF serializers do three jobs: parsing, validating, and shaping the response. In GoFr each is explicit:

- **Parsing** — `c.Bind(&dto)` for JSON / form / multipart.
- **Validating** — pair the bound struct with `go-playground/validator` (tag-based) or write checks in the handler.
- **Shaping** — return a typed struct; the response is the struct.

```go
type CreateUser struct {
    Name  string `json:"name"  validate:"required,min=3"`
    Email string `json:"email" validate:"required,email"`
}
```

## ORM to SQL drivers

This is the largest mental shift. GoFr does not ship an ORM. You write SQL — typically via `c.SQL.Query` / `Exec` — and pair it with `sqlc` if you want type-safe generated code, or `gorm` if you want ORM-like ergonomics. Both work fine inside GoFr handlers.

Plan to replace queryset chains with explicit SQL. Migrate the data model with [GoFr SQL migrations](/docs/advanced-guide/handling-data-migrations) — versioned files applied at boot.

## Permissions and auth

DRF's `permission_classes` map to a combination of GoFr authentication middleware (Basic, API Key, OAuth-JWT) and RBAC. See [authentication](/docs/advanced-guide/authentication). Per-request user identity is available via the request context.

## Pagination and filtering

DRF's `PageNumberPagination` / `LimitOffsetPagination` and DjangoFilterBackend don't have a built-in equivalent. The idiom is explicit:

```go
page := c.Param("page")
limit := c.Param("limit")
// translate to LIMIT/OFFSET in your SQL
```

This is honest extra work; the trade-off is no implicit query generation surprising you in production.

## Signals and async

Django signals (`post_save`, etc.) don't translate directly — they're an in-process pub/sub. The cross-service equivalent is GoFr Pub/Sub: emit a domain event from the handler, subscribe in another service.

Publish from inside a handler — `GetPublisher` is on `*gofr.Context`, and the payload must be `[]byte`:

```go
func handler(c *gofr.Context) (any, error) {
    if err := c.GetPublisher().Publish(c, "user.created", []byte(`{"id":"1"}`)); err != nil {
        return nil, err
    }
    return map[string]string{"status": "queued"}, nil
}
```

Subscribers (Kafka, NATS, SQS, MQTT, Google Pub/Sub, Azure Event Hub) are registered with `app.Subscribe`.

## Configuration

`settings.py` and `django-environ` → `configs/.env`, with `configs/.<APP_ENV>.env` overlaid on top (so `APP_ENV=production` reads `configs/.env` then `configs/.production.env` — note the dot prefix and `.env` suffix on the override file). Read keys in code with `app.Config.Get(key)`.

## Observability

DRF teams typically wire `django-prometheus`, `opentelemetry-instrumentation-django`, and structlog manually. GoFr emits OpenTelemetry traces, Prometheus metrics at `/metrics`, structured JSON logs (with trace IDs), and exposes health at `/.well-known/health`. Log levels can be changed at runtime via the [remote log-level endpoint](/docs/advanced-guide/remote-log-level-change).

## Gradual adoption

Stand up a GoFr service for one bounded context (e.g. notifications, search). From the Django side call it over HTTP; from GoFr call back into Django with `app.AddHTTPService("django-api", baseURL)` — circuit breaker, retries, and rate limiting included.

{% faq %}

{% faq-item question="Can I run Django and GoFr in the same cluster?" %}
Yes. They are independent services. Use Pub/Sub topics or HTTP to bridge; GoFr's HTTP service client adds resilience automatically.
{% /faq-item %}

{% faq-item question="Is there a Django admin equivalent?" %}
No. The CRUD surface is auto-generated via `AddRESTHandlers`, but a polished admin UI is out of scope — most teams build it separately or use a generic admin frontend pointed at the REST endpoints.
{% /faq-item %}

{% faq-item question="What about Celery beat schedules?" %}
GoFr's built-in cron scheduler covers periodic jobs; queue-driven work moves to Pub/Sub subscribers.
{% /faq-item %}

{% /faq %}
