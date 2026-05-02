---
description: "Run the same GoFr image across staging and production with APP_ENV, per-environment Helm values, isolated datasources, and environment-aware telemetry."
nextjs:
  metadata:
    title: "GoFr Multi-Environment Deployment: Staging to Production"
    description: "Run the same GoFr image across staging and production with APP_ENV, per-environment Helm values, isolated datasources, and environment-aware telemetry."
---

# Multi-Environment Deployment

{% answer %}
GoFr selects per-environment configuration through the `APP_ENV` env var, which picks `configs/.<APP_ENV>.env` at startup; in Kubernetes you ship one image and override every value through environment-specific ConfigMaps, Secrets, and Helm values files. Keep namespaces or clusters isolated, point each environment at its own database and tracing endpoint, and promote by tag — never by rebuilding for the target.
{% /answer %}

## When to use

Any time you have more than one running copy of a service — even if it's just `dev` and `prod` — you need a deployment story that prevents config drift. GoFr's twelve-factor config makes the *what* easy; this guide covers the *how* on Kubernetes.

## One image, many environments

The build artifact never changes between environments. The same image digest that ran in staging for a day promotes to production. Everything that varies — connection strings, log levels, feature flags, replica count, resource limits — comes from the cluster.

```text
git push tag v1.4.2 ──▶ CI builds image, signs, pushes
                  ──▶ deploy to staging  (APP_ENV=staging)
                  ──▶ smoke + soak
                  ──▶ deploy to prod     (APP_ENV=prod)
```

GoFr reads `APP_ENV` to decide which override file to overlay on `configs/.env`. In Kubernetes, the override file is largely vestigial — every value comes from a `ConfigMap` or `Secret` injected with `envFrom` (see [Twelve-Factor Config](/docs/advanced-guide/twelve-factor-config)). `APP_ENV` still matters because GoFr logs it on startup and you can branch app behavior on it: `if app.Config.Get("APP_ENV") == "prod" { ... }`.

## Namespace per env vs cluster per env

**Namespace per env** (`staging`, `prod` in the same cluster) is cheaper and simpler, but shares a control plane and nodes — a runaway prod workload can starve staging, and compliance frameworks often reject it for regulated data. **Cluster per env** isolates everything but doubles operational overhead. Most teams start with namespaces and graduate to separate prod clusters once compliance or noisy-neighbor pressure forces the move. Whichever you pick, never share the same database, broker, or tracing backend across envs.

## Helm values per environment

Keep one chart, one `values.yaml` for defaults, and one overrides file per env. Per-env files override only what's different — replica count, log level, datasource hosts.

```yaml
# values.yaml
replicaCount: 2
image: { repository: ghcr.io/example/orders-api }
config:
  HTTP_PORT: "8000"
  METRICS_PORT: "2121"
  LOG_LEVEL: INFO
  TRACE_EXPORTER: otlp
  SHUTDOWN_GRACE_PERIOD: 30s
```

```yaml
# values-staging.yaml
image: { tag: 1.4.2 }
config:
  APP_ENV: staging
  LOG_LEVEL: DEBUG
  DB_HOST: postgres.staging.svc.cluster.local
  TRACER_URL: http://otel-collector.observability.svc:4318
```

```yaml
# values-prod.yaml
replicaCount: 10
image: { tag: 1.4.2 }
config:
  APP_ENV: prod
  DB_HOST: postgres-primary.prod.svc.cluster.local
  TRACER_URL: http://otel-collector.observability.svc:4318
  DB_MAX_OPEN_CONNECTION: "20"
```

Apply with `helm upgrade --install orders-api ./chart -n prod -f values.yaml -f values-prod.yaml`. Same chart, same image tag, different values → different environment.

## Promotion flow

CI tags an image (`1.4.2`). `helm upgrade` deploys it to staging; after integration tests and a soak window, the same `1.4.2` tag promotes to prod. If a problem surfaces, `helm rollback orders-api -n prod` reverts. Never `docker build` again between envs — that invalidates the artifact you tested.

## Datasource separation

Each environment must point at its own datasources. Sharing a database across staging and prod is a data-corruption incident waiting to happen — staging migrations can drop columns prod still reads.

- Separate `DB_HOST` / `DB_NAME` per env.
- Separate Pub/Sub topics or namespaces (Kafka cluster + topic prefix, NATS account, MQTT broker).
- Separate Redis instances or at least separate `REDIS_DB` numbers.
- Separate object storage buckets.

For databases under heavy migration churn, give staging its own writable replica with a nightly snapshot from prod — close enough to be representative, isolated enough to be safe. See [Handling Data Migrations](/docs/advanced-guide/handling-data-migrations) for the migration story itself.

## Telemetry segregation

Tag every signal with the environment so dashboards and alerts can filter. Set a different `TRACER_URL` per env, or share a collector with an `env` resource attribute; use `TRACER_RATIO` (default 1) to drop prod sampling if volume is too high. Use `LOG_LEVEL=DEBUG` in staging, `INFO` in prod, and toggle without redeploying via `REMOTE_LOG_URL` (see [Remote Log Level Change](/docs/advanced-guide/remote-log-level-change)). Add an `env` Prometheus label via your scrape config so the same alert rule can fire per-environment with different thresholds. Staging alerts should page a chat channel; prod alerts page on-call.

## Verification

```bash
kubectl exec -n prod deploy/orders-api -- env | grep -E '^(APP_ENV|DB_HOST|TRACER_URL|LOG_LEVEL)='
curl https://orders-api.prod.example.com/.well-known/health
```

The first command verifies the running container actually has the env you expect; the second confirms the service is reachable.

{% faq %}
{% faq-item question="What is the env var name for selecting the environment in GoFr?" %}
`APP_ENV`. GoFr uses it to overlay `configs/.<APP_ENV>.env` on top of `configs/.env`, and you can read it from application code via `app.Config.Get("APP_ENV")`.
{% /faq-item %}
{% faq-item question="Should staging and production share a database?" %}
No. Migrations applied in staging can break the schema prod relies on, and any data leakage is a compliance incident. Always run separate databases (or separate writable instances of the same engine).
{% /faq-item %}
{% faq-item question="How do I change log level without redeploying?" %}
Set `REMOTE_LOG_URL` to a control-plane endpoint and adjust the level there — GoFr polls on `REMOTE_LOG_FETCH_INTERVAL` (default 15s). See the Remote Log Level Change guide.
{% /faq-item %}
{% /faq %}
