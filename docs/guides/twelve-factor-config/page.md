---
description: "Wire GoFr's Config interface to twelve-factor environment variables, .env files, and Kubernetes ConfigMap and Secret resources without leaking credentials."
nextjs:
  metadata:
    title: "GoFr Twelve-Factor Config: Env Vars, ConfigMaps, Secrets"
    description: "Wire GoFr's Config interface to twelve-factor environment variables, .env files, and Kubernetes ConfigMap and Secret resources without leaking credentials."
---

# Twelve-Factor Config in GoFr

{% answer %}
GoFr's `config.Config` interface reads from process environment variables and `.env` files in the `configs/` directory, with system env vars taking precedence over file values. In Kubernetes, ship the same binary across environments and inject configuration through `envFrom` referencing a `ConfigMap` (non-secret) and a `Secret` (credentials), keeping secrets out of source control.
{% /answer %}

## When to use

Twelve-factor config matters whenever the same artifact runs in more than one place — local laptop, CI, staging, production. GoFr is designed around this from the start: the framework itself is configured by env vars (`HTTP_PORT`, `DB_DIALECT`, `LOG_LEVEL`, etc.), and `app.Config.Get(...)` exposes the same surface to your application code.

## How GoFr loads config

The default loader is `config.NewEnvFile(configFolder, logger)` and the precedence is:

1. **System environment variables** — values present in `os.Environ()` *before* the app starts win.
2. **`configs/.env`** — base values for every environment.
3. **`configs/.<APP_ENV>.env`** — overrides for the named env (e.g., `configs/.staging.env` when `APP_ENV=staging`). Falls back to `configs/.local.env` when `APP_ENV` is unset.

The loader actually re-applies the captured initial environment after reading the override file, which is what guarantees system env > file. In a Kubernetes pod, every value injected via `env:` or `envFrom:` is a system env var and therefore beats anything baked into the `configs/` folder of the image.

The `Config` interface itself is small:

```go
type Config interface {
    Get(string) string
    GetOrDefault(string, string) string
}
```

Use it from any handler or service:

```go
threshold := app.Config.GetOrDefault("PAYMENT_RETRY_THRESHOLD", "3")
```

## Local development with `.env`

Keep a checked-in `configs/.env` with safe defaults and a gitignored `configs/.local.env` for personal overrides:

```dotenv
# configs/.env
APP_NAME=orders-api
HTTP_PORT=8000
LOG_LEVEL=DEBUG
DB_DIALECT=postgres
DB_HOST=localhost
DB_PORT=5432
DB_NAME=orders_dev
```

```dotenv
# configs/.local.env  (gitignored)
DB_PASSWORD=local-dev-password
```

When `APP_ENV` is unset GoFr loads `.env` then overlays `.local.env`. Set `APP_ENV=staging` and it overlays `.staging.env` instead.

## Kubernetes: ConfigMap + Secret

In production, the `configs/` directory inside the image is largely empty (or only holds non-environmental files like a GraphQL schema). Everything environmental comes from Kubernetes:

```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: orders-api-config
  namespace: prod
data:
  APP_NAME: orders-api
  APP_ENV: prod
  HTTP_PORT: "8000"
  METRICS_PORT: "2121"
  LOG_LEVEL: INFO
  DB_DIALECT: postgres
  DB_HOST: postgres-primary.prod.svc.cluster.local
  DB_PORT: "5432"
  DB_NAME: orders
  DB_MAX_OPEN_CONNECTION: "20"
  DB_MAX_IDLE_CONNECTION: "5"
  TRACE_EXPORTER: otlp
  # GoFr's OTLP exporter speaks gRPC (otlptracegrpc). TRACER_URL must be a bare
  # host:port — no http:// scheme — and the OTLP gRPC port is 4317 (4318 is OTLP
  # HTTP, which GoFr does NOT use).
  TRACER_URL: otel-collector.observability.svc.cluster.local:4317
---
apiVersion: v1
kind: Secret
metadata:
  name: orders-api-secret
  namespace: prod
type: Opaque
stringData:
  DB_USER: orders_app
  DB_PASSWORD: replace-me
```

Wire both into the Deployment with `envFrom` so every key becomes an env var without listing them individually:

```yaml
spec:
  template:
    spec:
      containers:
        - name: api
          image: ghcr.io/example/orders-api:1.4.2
          envFrom:
            - configMapRef:
                name: orders-api-config
            - secretRef:
                name: orders-api-secret
          ports:
            - name: http
              containerPort: 8000
            - name: metrics
              containerPort: 2121
```

If both the ConfigMap and Secret define the same key, the *later* `envFrom` entry wins — list the Secret last for credentials that must override defaults.

## Secret management

Don't commit `Secret` manifests with real values to Git. Two well-supported options:

- {% new-tab-link newtab=true title="Sealed Secrets" href="https://sealed-secrets.netlify.app/" /%} — encrypt the Secret manifest with a controller-held key; safe to commit.
- {% new-tab-link newtab=true title="External Secrets Operator" href="https://external-secrets.io/" /%} — sync from Vault, AWS Secrets Manager, GCP Secret Manager, etc.

GoFr does not need to know which one you use; both materialize a normal `Secret` that `envFrom` consumes.

## When to use the `configs/` folder vs env

Use **env vars** for anything that varies by environment: hostnames, ports, log levels, feature flags, credentials.

Use the **`configs/` folder** for static assets the binary needs at runtime: a GraphQL `schema.graphql`, an OpenAPI `openapi.json` (which GoFr auto-mounts as Swagger UI when present), or a fixed routing table. Bake these into the image — they don't change between staging and prod.

## Anti-patterns

- Hardcoded URLs (`"http://payments.internal"`) — breaks the moment staging needs a different host.
- Secrets committed to Git, even in a private repo — they leak via clones, CI artifacts, and IDE history.
- Reading `os.Getenv` directly in handlers — use `app.Config.Get` so tests can substitute a mock `Config`.
- One ConfigMap that mixes secrets with non-secrets — defeats the point of using a Secret resource for RBAC and audit.

{% faq %}
{% faq-item question="What is the precedence between .env files and process environment in GoFr?" %}
System environment variables always win. GoFr captures `os.Environ()` before loading files and re-applies it after `godotenv.Overload`, so anything injected by Kubernetes overrides what's in `configs/.env` and `configs/.<APP_ENV>.env`.
{% /faq-item %}
{% faq-item question="Where do I set APP_ENV?" %}
Anywhere your platform supplies env vars: a `ConfigMap` key in Kubernetes, the shell in CI, or `configs/.local.env` for development. GoFr reads it on startup to pick the override file.
{% /faq-item %}
{% faq-item question="Can I use Vault or AWS Secrets Manager?" %}
Yes, indirectly. Use External Secrets Operator (or a sidecar) to materialize a Kubernetes `Secret`, then reference it with `envFrom`. GoFr only sees env vars and doesn't care about the source.
{% /faq-item %}
{% /faq %}
