---
description: "Dockerize a GoFr Go microservice with a multi-stage, distroless, non-root image and a healthcheck on /.well-known/alive."
nextjs:
  metadata:
    title: "Dockerizing GoFr Services - Multi-Stage Distroless Build"
    description: "Dockerize a GoFr Go microservice with a multi-stage, distroless, non-root image and a healthcheck on /.well-known/alive."
---

# Dockerizing GoFr Services

{% answer %}
A production GoFr container is a multi-stage build: compile a static, CGO-disabled binary in a `golang:alpine` builder stage, then copy it into a `gcr.io/distroless/static-debian12:nonroot` runtime image. Docker reads configuration from environment variables (`HTTP_PORT`, `METRICS_PORT`, datasource URLs), and the built-in `/.well-known/alive` endpoint backs the container `HEALTHCHECK`.
{% /answer %}

## When to use this guide

Use this guide when you have a GoFr service running locally with `go run` and need to package it for a registry, CI, or Kubernetes. The output is a small (typically under 20 MB), non-root image that does not ship a shell or package manager — keeping the attack surface small for production.

For Kubernetes manifests that consume this image, see {% new-tab-link newtab=false title="Deploying to Kubernetes" href="/docs/guides/deploying-to-kubernetes" /%}.

## Project layout

A typical containerized GoFr project looks like this:

```text
my-service/
├── main.go
├── go.mod
├── go.sum
├── configs/
│   └── .env
├── Dockerfile
├── .dockerignore
└── docker-compose.yml
```

GoFr loads `configs/.env` automatically when present, but in containers you should prefer real environment variables — that is what Kubernetes ConfigMaps and Secrets inject.

## Multi-stage Dockerfile

Save this as `Dockerfile` at the repo root:

```dockerfile
# syntax=docker/dockerfile:1.7

ARG GO_VERSION=1.25
ARG APP_VERSION=dev
ARG GIT_COMMIT=unknown

# ---------- builder ----------
FROM golang:${GO_VERSION}-alpine AS builder

RUN apk add --no-cache git ca-certificates tzdata

WORKDIR /src

# Cache module downloads in their own layer.
COPY go.mod go.sum ./
RUN go mod download

# Copy source after deps so source edits don't bust the dep cache.
COPY . .

ARG APP_VERSION
ARG GIT_COMMIT

# CGO=0 + -trimpath gives a static, reproducible binary.
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 \
    go build \
      -trimpath \
      -ldflags="-s -w -X main.version=${APP_VERSION} -X main.commit=${GIT_COMMIT}" \
      -o /out/app ./

# ---------- runtime ----------
FROM gcr.io/distroless/static-debian12:nonroot

WORKDIR /app

COPY --from=builder /out/app /app/app
# Copy .env only if you want a baked-in default; in K8s prefer ConfigMaps.
COPY --from=builder /src/configs /app/configs

USER nonroot:nonroot

EXPOSE 8000 2121

# /.well-known/alive is registered by GoFr automatically and returns 200 when the process is up.
# NOTE: distroless/static has no shell and no wget/curl, so a Dockerfile HEALTHCHECK is
# impractical on this base image. The recommended path is to omit HEALTHCHECK and rely on
# Kubernetes liveness/readiness probes (see the Deploying to Kubernetes guide). If you do
# need a docker-level healthcheck, switch the runtime stage to a base image that includes
# wget (e.g. alpine) and use:
#   HEALTHCHECK --interval=10s --timeout=2s --retries=5 \
#     CMD wget -qO- http://localhost:8000/.well-known/alive || exit 1
# There is NO `healthcheck` subcommand on the GoFr binary — `/app/app healthcheck` will
# re-run the whole app and fail with a port-bind error.

ENTRYPOINT ["/app/app"]
```

A few things worth calling out:

- **`CGO_ENABLED=0`** produces a static binary — required because `distroless/static` has no glibc.
- **`distroless/static-debian12:nonroot`** ships only the binary, CA certs, `/etc/passwd`, and timezone data. No shell, no package manager.
- **`USER nonroot`** runs as UID 65532; combined with a read-only root filesystem in Kubernetes this satisfies most pod-security baselines.
- **Healthchecks** rely on the `/.well-known/alive` endpoint that GoFr registers automatically (see {% new-tab-link newtab=false title="Monitoring Service Health" href="/docs/advanced-guide/monitoring-service-health" /%}). There is no `healthcheck` subcommand on the GoFr binary, and `distroless/static` has no shell or `wget`/`curl` to call the endpoint, so a Dockerfile `HEALTHCHECK` directive does not work cleanly on this base image. On Kubernetes the standard path is to skip `HEALTHCHECK` entirely and use the Deployment's `livenessProbe`/`readinessProbe` (which `httpGet` the same `/.well-known/alive` and `/.well-known/health` paths). If you need a docker-level healthcheck, switch the runtime stage to a base image that includes `wget` (for example `alpine`) and use the commented form shown above.

## .dockerignore

Without this, `COPY . .` pulls in `.git`, local secrets, and build artifacts:

```text
.git
.gitignore
.dockerignore
Dockerfile
docker-compose.yml
*.md
**/*_test.go
bin/
dist/
configs/.env.local
.env
.env.*
```

## Building and tagging

```bash
docker build \
  --build-arg APP_VERSION=$(git describe --tags --always) \
  --build-arg GIT_COMMIT=$(git rev-parse --short HEAD) \
  -t my-org/my-service:$(git rev-parse --short HEAD) \
  -t my-org/my-service:latest \
  .
```

Always tag with a commit SHA in addition to (or instead of) `latest`. Kubernetes `RollingUpdate` only rolls when the image reference actually changes, and `latest` is mutable.

## docker-compose for local development

For local dev you usually want the service plus a few datasources. This compose file matches GoFr's default ports (HTTP `8000`, metrics `2121`):

```yaml
services:
  app:
    build: .
    ports:
      - "8000:8000"
      - "2121:2121"
    environment:
      APP_NAME: my-service
      HTTP_PORT: "8000"
      METRICS_PORT: "2121"
      LOG_LEVEL: DEBUG
      REDIS_HOST: redis
      REDIS_PORT: "6379"
      DB_HOST: postgres
      DB_PORT: "5432"
      DB_USER: gofr
      DB_PASSWORD: gofr
      DB_NAME: gofr
      DB_DIALECT: postgres
      PUBSUB_BACKEND: KAFKA
      PUBSUB_BROKER: kafka:9092
    depends_on:
      - redis
      - postgres
      - kafka

  redis:
    image: redis:7-alpine
    ports: ["6379:6379"]

  postgres:
    image: postgres:16-alpine
    environment:
      POSTGRES_USER: gofr
      POSTGRES_PASSWORD: gofr
      POSTGRES_DB: gofr
    ports: ["5432:5432"]

  kafka:
    image: bitnami/kafka:3.7
    environment:
      KAFKA_CFG_NODE_ID: "0"
      KAFKA_CFG_PROCESS_ROLES: controller,broker
      KAFKA_CFG_CONTROLLER_QUORUM_VOTERS: "0@kafka:9093"
      KAFKA_CFG_LISTENERS: PLAINTEXT://:9092,CONTROLLER://:9093
      KAFKA_CFG_ADVERTISED_LISTENERS: PLAINTEXT://kafka:9092
      KAFKA_CFG_CONTROLLER_LISTENER_NAMES: CONTROLLER
    ports: ["9092:9092"]
```

The exact env var names for each datasource (Mongo, Cassandra, etc.) are documented under {% new-tab-link newtab=false title="Injecting Databases Drivers" href="/docs/advanced-guide/injecting-databases-drivers" /%}.

## Production tips

- **Image size:** with `distroless/static`, a typical GoFr binary lands at 15–25 MB compressed. If you see hundreds of MB, you forgot `CGO_ENABLED=0` or copied build artifacts.
- **Read-only root FS:** in Kubernetes, set `readOnlyRootFilesystem: true` and mount an `emptyDir` if the service writes temp files.
- **Don't bake secrets:** never `COPY` a populated `.env` into the runtime image. Inject via Kubernetes Secrets instead.
- **Pin the Go version:** the `ARG GO_VERSION` lets CI build the same image deterministically.
- **Build cache:** add `--mount=type=cache,target=/go/pkg/mod` with BuildKit to keep module cache warm between CI runs.

## Verification

```bash
# Build and run.
docker build -t my-service:dev .
docker run --rm -p 8000:8000 -p 2121:2121 my-service:dev

# In another shell:
curl -s http://localhost:8000/.well-known/alive
# {"data":{"status":"UP"}}

curl -s http://localhost:2121/metrics | head
# # HELP app_http_response ...
# # TYPE app_http_response histogram

# Inspect image size and layers.
docker image inspect my-service:dev --format '{{.Size}}'
docker history my-service:dev
```

{% faq %}
{% faq-item question="Why distroless instead of alpine?" %}
Alpine includes BusyBox, apk, and a shell — useful for debugging but extra attack surface. Distroless ships only what your binary needs, so CVEs in shells and package managers cannot affect you. If you need to debug, run `docker run --rm -it --entrypoint sh` against the *builder* stage instead of the runtime image.
{% /faq-item %}
{% faq-item question="Can I use scratch instead of distroless?" %}
Yes — `FROM scratch` is even smaller, but you must `COPY` `/etc/ssl/certs/ca-certificates.crt` yourself for HTTPS to work. Distroless includes that plus `nonroot` user mappings, which is why it is the default recommendation here.
{% /faq-item %}
{% faq-item question="How do I run database migrations on container start?" %}
Use a Kubernetes Job, an init container that runs the same image with a different argument, or wire migrations into a GoFr `OnStart` hook (see {% new-tab-link newtab=false title="Startup Hooks" href="/docs/advanced-guide/startup-hooks" /%}). Migrations on every replica on startup is racy under HPA.
{% /faq-item %}
{% /faq %}
