---
description: "Dockerize a GoFr Go microservice two ways — multi-stage distroless build, or copy a CI-built binary into a distroless non-root runtime."
nextjs:
  metadata:
    title: "Dockerizing GoFr Services - Multi-Stage and Copy-Binary Variants"
    description: "Dockerize a GoFr Go microservice two ways — multi-stage distroless build, or copy a CI-built binary into a distroless non-root runtime."
---

# Dockerizing GoFr Services

{% answer %}
Two production-ready ways to ship GoFr in a container: a multi-stage build that compiles a static, CGO-disabled binary inside the image, or a copy-binary variant that lifts a CI-built binary into a minimal runtime. Both target `gcr.io/distroless/static-debian12:nonroot`, expose `HTTP_PORT` (8000) and `METRICS_PORT` (2121), read configuration from env vars, and rely on Kubernetes liveness/readiness probes (the `/.well-known/alive` and `/.well-known/health` endpoints GoFr registers) — Dockerfile `HEALTHCHECK` does not work cleanly on distroless.
{% /answer %}

{% howto name="Containerize a GoFr service (multi-stage)" description="Build a small, secure container image for a GoFr binary using a multi-stage Go build." steps=[{"name": "Add a multi-stage Dockerfile", "text": "Use a golang:1.25-alpine builder stage to compile a static binary, then copy it into a gcr.io/distroless/static-debian12:nonroot runtime stage."}, {"name": "Cache module downloads", "text": "COPY go.mod and go.sum first and run go mod download before copying source — combined with a BuildKit cache mount, Docker reuses module downloads across builds."}, {"name": "Compile a static binary", "text": "Set CGO_ENABLED=0 with -trimpath and use -ldflags to embed version/commit, so the runtime image needs no libc and stays minimal."}, {"name": "Run as non-root", "text": "distroless/static-debian12:nonroot already provides UID 65532; set USER nonroot:nonroot to use it."}, {"name": "Probe over HTTP from outside the container", "text": "distroless/static has no shell or wget/curl, so Dockerfile HEALTHCHECK is impractical. On Kubernetes, use livenessProbe/readinessProbe with httpGet on /.well-known/alive and /.well-known/health."}, {"name": "Build and tag", "text": "docker build with a short-SHA tag plus a semver alias so you can roll back by digest."}] /%}

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

## Choose your variant

Two production-ready paths. Pick based on where you want compilation to happen.

| Variant | When to prefer |
| --- | --- |
| Multi-stage build | You want a single `docker build` to produce a release-grade image. Build context lives entirely in-repo. |
| Copy pre-built binary | Your CI already produces a reproducible binary (e.g., signed/attested by SLSA, GoReleaser, etc.). The image build is a thin wrapper around that artifact, so it's faster and the build context is tiny. |

## Variant A: Multi-stage Dockerfile

Save this as `Dockerfile` at the repo root:

```dockerfile
# syntax=docker/dockerfile:1.7

ARG GO_VERSION=1.25
ARG APP_VERSION=dev
ARG GIT_COMMIT=unknown

# ---------- builder ----------
FROM --platform=$BUILDPLATFORM golang:${GO_VERSION}-alpine AS builder

RUN apk add --no-cache git ca-certificates

WORKDIR /src

# Cache module downloads in their own layer.
COPY go.mod go.sum ./
RUN --mount=type=cache,target=/go/pkg/mod \
    go mod download

# Copy source after deps so source edits don't bust the dep cache.
COPY . .

ARG APP_VERSION
ARG GIT_COMMIT
ARG TARGETOS
ARG TARGETARCH

# CGO=0 + -trimpath gives a static, reproducible binary.
# TARGETOS/TARGETARCH come from BuildKit so the same Dockerfile builds for
# linux/amd64 and linux/arm64 unchanged.
RUN --mount=type=cache,target=/go/pkg/mod \
    CGO_ENABLED=0 GOOS=${TARGETOS:-linux} GOARCH=${TARGETARCH:-amd64} \
    go build \
      -trimpath \
      -ldflags="-s -w -X main.version=${APP_VERSION} -X main.commit=${GIT_COMMIT}" \
      -o /out/app ./

# ---------- runtime ----------
FROM gcr.io/distroless/static-debian12:nonroot

WORKDIR /app

COPY --from=builder /out/app /app/app

USER nonroot:nonroot

EXPOSE 8000 2121

# distroless/static has no shell and no wget/curl, so a Dockerfile HEALTHCHECK
# is impractical here. On Kubernetes, use the Deployment's livenessProbe and
# readinessProbe (httpGet on /.well-known/alive and /.well-known/health) — see
# the Deploying to Kubernetes guide.

ENTRYPOINT ["/app/app"]
```

A few things worth calling out:

- **`CGO_ENABLED=0`** produces a fully statically-linked binary with no dependency on `libc` or a dynamic linker at runtime — required because `distroless/static-debian12:nonroot` ships only the binary, CA certs, `/etc/passwd`, tzdata, and a non-root user. There is no `libc` (glibc, musl, anything), no shell, no package manager.
- **`TARGETOS` / `TARGETARCH` ARGs** let one Dockerfile build for `linux/amd64` and `linux/arm64` via `docker buildx build --platform=linux/amd64,linux/arm64 …` — useful when developing on Apple Silicon and deploying to amd64 nodes (or vice versa).
- **`-X main.version=…`** ldflags only inject values if your `main` package declares matching variables. Add `var (version, commit string)` near the top of `main.go` if you want `gofr.Logger().Info(version, commit)` to surface the build's git SHA.
- **`USER nonroot`** runs as UID 65532; combined with a read-only root filesystem in Kubernetes this satisfies most pod-security baselines.
- **No bundled `configs/`**: env vars come from the platform (compose, K8s ConfigMap/Secret, cloud SSM/Secrets Manager). Do not `COPY configs/` into the runtime image — it tends to drift, and a populated `.env` is a secret. Bake only platform-independent defaults into your binary.
- **Healthchecks** rely on `/.well-known/alive` (process up) and `/.well-known/health` (datasources reachable) that GoFr registers automatically. There is no `healthcheck` subcommand on the GoFr binary, and `distroless/static` has no shell or `wget`/`curl` to call the endpoint, so a Dockerfile `HEALTHCHECK` directive does not work cleanly on this base. On Kubernetes, use the Deployment's `livenessProbe` / `readinessProbe` instead (see the Deploying to Kubernetes guide).

## Variant B: Copy a pre-built binary

If your CI already produces a release-grade Go binary — reproducible flags, SLSA provenance, signed by cosign, whatever your supply chain looks like — you don't need a Go toolchain inside the image. Lift the binary in.

Build the binary in CI:

```bash
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 \
  go build -trimpath -ldflags='-s -w' -o ./bin/app ./
```

Then this is the entire Dockerfile:

```dockerfile
# syntax=docker/dockerfile:1.7

FROM gcr.io/distroless/static-debian12:nonroot

WORKDIR /app

# `./bin/app` is the binary your CI produced one step earlier.
COPY ./bin/app /app/app

USER nonroot:nonroot

EXPOSE 8000 2121

ENTRYPOINT ["/app/app"]
```

Why this is sometimes preferable:

- **Faster image builds**: no Go toolchain, no module download, no compile step. The image build is a single `COPY`.
- **Smaller build context**: `docker build` only needs `./bin/app` and the Dockerfile. Use a tight `.dockerignore` (or build with a custom context) so source isn't shipped to the daemon.
- **Decoupled supply chain**: the binary and its provenance are signed once in CI and the image build never touches source. This matches SLSA Level 3+ patterns.

When NOT to use this variant:

- You want a single `docker build` to be the only entry-point for a fresh checkout. Variant A is more self-contained.
- You're shipping arch-specific binaries from the same Dockerfile. Variant A's `TARGETARCH` flow is cleaner.

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
- **Build cache:** Variant A's Dockerfile already includes the `--mount=type=cache,target=/go/pkg/mod` cache mount on both `go mod download` and `go build`; just use BuildKit (default in `docker buildx`, or set `DOCKER_BUILDKIT=1`) to keep the module cache warm between CI runs.

## Verification

A hello-world GoFr service (no datasources) needs no env injection:

```bash
docker build -t my-service:dev .
docker run --rm -p 8000:8000 -p 2121:2121 my-service:dev

# In another shell:
curl -s http://localhost:8000/.well-known/alive
# {"data":{"status":"UP"}}

curl -s http://localhost:2121/metrics | head
# # HELP app_http_response ...
# # TYPE app_http_response histogram
```

A real service with datasources needs env vars. Use `--env-file`:

```bash
cat > .env.dev <<'EOF'
APP_NAME=my-service
HTTP_PORT=8000
METRICS_PORT=2121
LOG_LEVEL=DEBUG
REDIS_HOST=host.docker.internal
REDIS_PORT=6379
DB_HOST=host.docker.internal
DB_PORT=5432
DB_USER=gofr
DB_PASSWORD=gofr
DB_NAME=gofr
DB_DIALECT=postgres
EOF

docker run --rm -p 8000:8000 -p 2121:2121 --env-file .env.dev my-service:dev

# Same curl checks as above.

# Inspect image size and layers:
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
