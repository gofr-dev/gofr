---
description: "Deploy a GoFr Go microservice to Kubernetes with probes wired to /.well-known/health and /.well-known/alive, plus ConfigMap, Secret, and HPA."
nextjs:
  metadata:
    title: "Deploying GoFr to Kubernetes - Probes, ConfigMap, HPA"
    description: "Deploy a GoFr Go microservice to Kubernetes with probes wired to /.well-known/health and /.well-known/alive, plus ConfigMap, Secret, and HPA."
---

# Deploying GoFr to Kubernetes

{% answer %}
Deploy a GoFr service to Kubernetes by pointing the readiness probe at `/.well-known/health`, the liveness probe at `/.well-known/alive`, and feeding non-secret config through a ConfigMap (`envFrom`) and credentials through a Secret. Set `terminationGracePeriodSeconds` higher than the longest in-flight request so GoFr's graceful shutdown can drain cleanly.
{% /answer %}

## When to use this guide

You have a GoFr service already containerized (see {% new-tab-link newtab=false title="Dockerizing GoFr Services" href="/docs/advanced-guide/dockerizing-gofr-services" /%}) and a Kubernetes cluster (kind, EKS, GKE, AKS, or on-prem). This guide covers the manifest set for a stateless HTTP service: Deployment, Service, ConfigMap, Secret, and an optional HorizontalPodAutoscaler.

## How GoFr features map to Kubernetes resources

| GoFr feature | Kubernetes object | Notes |
|---|---|---|
| `/.well-known/alive` | `livenessProbe.httpGet` | Restart unhealthy pods |
| `/.well-known/health` | `readinessProbe.httpGet` | Gate traffic until datasources are reachable |
| `OnStart` hooks | `startupProbe` | Long warm-ups (cache fill, migrations) |
| Graceful shutdown on SIGTERM | `terminationGracePeriodSeconds` | Drain in-flight requests |
| `configs/.env` keys | `ConfigMap` + `envFrom` | Non-secret config |
| DB passwords, API keys | `Secret` + `envFrom` | Mount via env, not files |
| `/metrics` (port 2121) | named container port + ServiceMonitor | See {% new-tab-link newtab=false title="Production Prometheus on Kubernetes" href="/docs/advanced-guide/production-prometheus-kubernetes" /%} |

## Full manifest set

The following manifests deploy a GoFr service named `orders` listening on HTTP `8000` and Prometheus `2121`. Save them in a `k8s/` directory and apply with `kubectl apply -f k8s/`.

### ConfigMap (non-secret config)

```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: orders-config
  namespace: default
data:
  APP_NAME: "orders"
  HTTP_PORT: "8000"
  METRICS_PORT: "2121"
  LOG_LEVEL: "INFO"
  TRACE_EXPORTER: "otlp"
  TRACER_URL: "otel-collector.observability.svc.cluster.local:4317"
  TRACER_RATIO: "0.1"
  REDIS_HOST: "redis.default.svc.cluster.local"
  REDIS_PORT: "6379"
  DB_HOST: "postgres.default.svc.cluster.local"
  DB_PORT: "5432"
  DB_NAME: "orders"
  DB_DIALECT: "postgres"
```

These keys are read by GoFr from environment variables — the same names you use in `configs/.env` locally.

### Secret (credentials)

```yaml
apiVersion: v1
kind: Secret
metadata:
  name: orders-secret
  namespace: default
type: Opaque
stringData:
  DB_USER: "orders_app"
  DB_PASSWORD: "change-me"
  REDIS_PASSWORD: "change-me"
```

For real clusters, generate this with `kubectl create secret generic ... --from-literal=...` or use an external secrets operator (Vault, AWS Secrets Manager, etc.). Never commit populated Secret YAML.

### Deployment

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: orders
  namespace: default
  labels:
    app.kubernetes.io/name: orders
spec:
  replicas: 3
  revisionHistoryLimit: 5
  strategy:
    type: RollingUpdate
    rollingUpdate:
      maxSurge: 25%
      maxUnavailable: 0
  selector:
    matchLabels:
      app.kubernetes.io/name: orders
  template:
    metadata:
      labels:
        app.kubernetes.io/name: orders
    spec:
      terminationGracePeriodSeconds: 45
      securityContext:
        runAsNonRoot: true
        runAsUser: 65532
        seccompProfile:
          type: RuntimeDefault
      containers:
        - name: orders
          image: my-org/orders:1.4.2
          imagePullPolicy: IfNotPresent
          ports:
            - name: http
              containerPort: 8000
              protocol: TCP
            - name: metrics
              containerPort: 2121
              protocol: TCP
          envFrom:
            - configMapRef:
                name: orders-config
            - secretRef:
                name: orders-secret
          resources:
            requests:
              cpu: "200m"
              memory: "256Mi"
            limits:
              cpu: "1"
              memory: "512Mi"
          livenessProbe:
            httpGet:
              path: /.well-known/alive
              port: http
            initialDelaySeconds: 5
            periodSeconds: 10
            timeoutSeconds: 2
            failureThreshold: 3
          readinessProbe:
            httpGet:
              path: /.well-known/health
              port: http
            initialDelaySeconds: 5
            periodSeconds: 5
            timeoutSeconds: 2
            failureThreshold: 3
          startupProbe:
            httpGet:
              path: /.well-known/alive
              port: http
            failureThreshold: 30
            periodSeconds: 2
          securityContext:
            allowPrivilegeEscalation: false
            readOnlyRootFilesystem: true
            capabilities:
              drop: ["ALL"]
```

The resource requests/limits above (`200m` / `256Mi` request, `1` / `512Mi` limit) are reasonable starting points for a small CRUD service, **not** a prescription. Profile your service under realistic load and adjust — a service that fans out to many datasources will use more memory; a CPU-bound JSON-heavy API may need a higher CPU limit.

### Service

```yaml
apiVersion: v1
kind: Service
metadata:
  name: orders
  namespace: default
  labels:
    app.kubernetes.io/name: orders
spec:
  type: ClusterIP
  selector:
    app.kubernetes.io/name: orders
  ports:
    - name: http
      port: 80
      targetPort: http
      protocol: TCP
    - name: metrics
      port: 2121
      targetPort: metrics
      protocol: TCP
```

Naming the metrics port `metrics` lets a Prometheus `ServiceMonitor` select it by name without hardcoding `2121`.

### HorizontalPodAutoscaler (optional)

```yaml
apiVersion: autoscaling/v2
kind: HorizontalPodAutoscaler
metadata:
  name: orders
  namespace: default
spec:
  scaleTargetRef:
    apiVersion: apps/v1
    kind: Deployment
    name: orders
  minReplicas: 3
  maxReplicas: 20
  metrics:
    - type: Resource
      resource:
        name: cpu
        target:
          type: Utilization
          averageUtilization: 70
```

For traffic-driven scaling, switch to a custom-metrics adapter against the `app_http_response` histogram GoFr exports (request rate or p95 latency).

## Probes: why `/.well-known/health` for readiness, `/.well-known/alive` for liveness?

Both endpoints are registered automatically by GoFr (see {% new-tab-link newtab=false title="Monitoring Service Health" href="/docs/advanced-guide/monitoring-service-health" /%}).

- **`/.well-known/alive`** returns 200 as long as the HTTP server is up. A failure means "the process is wedged — restart me." That maps to *liveness*.
- **`/.well-known/health`** returns 200 only when the service **and its dependencies** are reachable. A failure here means "I'm up but I can't serve traffic right now — stop sending it." That maps to *readiness*.

Using `/.well-known/health` for liveness is a common mistake: a transient Redis outage will then cause kubelet to restart pods in a loop, taking the service fully offline.

## Graceful shutdown

When Kubernetes terminates a pod it sends `SIGTERM`, removes the pod from the Service endpoints, and waits up to `terminationGracePeriodSeconds` before sending `SIGKILL`. GoFr's `app.Run()` listens for `SIGINT` and `SIGTERM` and stops accepting new requests while letting in-flight ones finish.

Set `terminationGracePeriodSeconds` to slightly more than your longest realistic request — `45` is a safe default for typical APIs; bump it for services that stream or batch. If you have `OnStart` warm-up logic, see {% new-tab-link newtab=false title="Startup Hooks" href="/docs/advanced-guide/startup-hooks" /%}.

## Production tips

- **`maxUnavailable: 0`** during rollouts is safer than the default `25%` — combined with `maxSurge: 25%`, you get zero-downtime deploys at the cost of one extra pod's worth of resources.
- **Pin image tags** to a SHA or semantic version. `:latest` will not roll the Deployment when you push a new image.
- **PodDisruptionBudget** with `minAvailable: 2` (or `maxUnavailable: 1`) protects you during node drains.
- **Don't put `/metrics` behind authentication** in-cluster — Prometheus must scrape it, and `NetworkPolicy` is a cleaner control.
- **Tracing sampling:** in production, `TRACER_RATIO=0.1` (10%) is a sensible starting point. See {% new-tab-link newtab=false title="Production Tracing" href="/docs/advanced-guide/production-tracing" /%}.

## Verification

```bash
kubectl apply -f k8s/

# Wait for rollout.
kubectl rollout status deployment/orders --timeout=120s

# Inspect probe state.
kubectl get pods -l app.kubernetes.io/name=orders
kubectl describe pod <pod-name> | grep -A2 -E "Liveness|Readiness|Startup"

# Hit the endpoints from inside the cluster.
kubectl run curl --rm -it --image=curlimages/curl --restart=Never -- \
  curl -s http://orders.default.svc.cluster.local/.well-known/health

# Or port-forward for local poking.
kubectl port-forward svc/orders 8080:80 2121:2121
curl -s http://localhost:8080/.well-known/health
curl -s http://localhost:2121/metrics | head
```

{% faq %}
{% faq-item question="My pod is CrashLoopBackOff right after deploy — how do I tell if it's a probe issue?" %}
`kubectl describe pod` shows the last container exit reason and recent probe failures. If liveness fired before the app finished initializing, raise `startupProbe.failureThreshold` (each unit is `periodSeconds`, so `30 * 2s = 60s` of grace). If readiness keeps failing, port-forward to the pod and `curl /.well-known/health` directly — the JSON body lists which dependency is down.
{% /faq-item %}
{% faq-item question="Should I run the metrics server on the same port as HTTP?" %}
GoFr binds metrics on `METRICS_PORT` (default `2121`) separately from `HTTP_PORT` (default `8000`). Keep them split so you can apply different `NetworkPolicy` rules — for example, only allow Prometheus to reach `2121`.
{% /faq-item %}
{% faq-item question="How do I roll over secrets without downtime?" %}
Update the Secret, then trigger a rollout with `kubectl rollout restart deployment/orders`. Pods come back with the new env values via `envFrom`. For automatic reload on Secret change, use a tool like Reloader, since `envFrom` doesn't auto-update running containers.
{% /faq-item %}
{% /faq %}
