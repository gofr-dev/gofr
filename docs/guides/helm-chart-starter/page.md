---
description: "A reference Helm chart for a GoFr microservice: Chart.yaml, values.yaml, deployment, service, _helpers.tpl, with probes pointing at GoFr defaults."
nextjs:
  metadata:
    title: "GoFr Helm Chart Starter: Reference Templates for Kubernetes"
    description: "A reference Helm chart for a GoFr microservice: Chart.yaml, values.yaml, deployment, service, _helpers.tpl, with probes pointing at GoFr defaults."
---

# Helm Chart Starter

{% answer %}
This is a copy-paste reference Helm chart for a GoFr microservice. It assumes the application listens on the framework defaults — HTTP 8000, gRPC 9000, metrics 2121 — and uses `/.well-known/alive` and `/.well-known/health` for probes. Use it as the starting point for your own chart.
{% /answer %}

{% howto name="Package a GoFr service as a Helm chart" description="Build a minimal Helm chart for a GoFr microservice with templated Deployment, Service, ConfigMap, and probe wiring." steps=[{"name": "Create the chart skeleton", "text": "Generate Chart.yaml with appVersion plus apiVersion v2 and a values.yaml capturing image, replicas, env, resources, and probes."}, {"name": "Template the Deployment", "text": "In templates/deployment.yaml render replicas, image, envFrom (ConfigMap + Secret), readinessProbe at /.well-known/health and livenessProbe at /.well-known/alive."}, {"name": "Template the Service", "text": "In templates/service.yaml expose port 8000 (HTTP) and 2121 (metrics) as named ports for Prometheus scraping."}, {"name": "Wire ConfigMap and Secret", "text": "Mount values.env via ConfigMap for non-secrets and a separate Secret for credentials; both via envFrom."}, {"name": "Lint and template", "text": "Run helm lint and helm template to verify YAML output before installing."}, {"name": "Install and upgrade", "text": "helm install for first deploy, then helm upgrade --install on subsequent rollouts; tag image by digest for repeatability."}] /%}

{% callout type="note" title="Prefer a maintained chart?" %}
The reference chart below is intentionally minimal so you can read every line. If you'd rather depend on a maintained chart, the community chart at [zop/service](https://github.com/zopdev/helm-charts/tree/main/charts/service) covers the same shape (Deployment + Service + optional Ingress/HPA + probes).

```bash
helm repo add zop https://helm.zop.dev
helm install my-app zop/service
```

Override values with `-f values.yaml` or `--set`; the chart's `values.schema.json` marks user-mutable fields with `"mutable": true`.
{% /callout %}

This is reference material, not a published chart. A future `gofr-dev/gofr-k8s-starter` repo could host a maintained version. For now, copy the files below into a `chart/` directory in your service repo.

## Layout

```text
chart/
├── Chart.yaml
├── values.yaml
└── templates/
    ├── _helpers.tpl
    ├── deployment.yaml
    └── service.yaml
```

## Chart.yaml

```yaml
apiVersion: v2
name: gofr-service
description: A reference Helm chart for a GoFr microservice
type: application
version: 0.1.0
appVersion: "0.1.0"
```

## values.yaml

```yaml
image:
  repo: ghcr.io/example/my-gofr-service
  tag: latest
  pullPolicy: IfNotPresent

replicaCount: 2

service:
  type: ClusterIP
  httpPort: 8000
  grpcPort: 9000
  metricsPort: 2121

resources:
  requests:
    cpu: 100m
    memory: 128Mi
  limits:
    cpu: 500m
    memory: 512Mi

env: {}
  # DB_HOST: db.svc
  # LOG_LEVEL: INFO
  # TRACE_EXPORTER: otlp
  # TRACER_URL: tempo:4317

envFromSecrets: []
  # - my-db-credentials

ingress:
  enabled: false
  className: nginx
  host: api.example.com
  tls:
    enabled: false
    secretName: api-tls

autoscaling:
  enabled: false
  minReplicas: 2
  maxReplicas: 10
  targetCPUUtilizationPercentage: 70

podSecurityContext:
  runAsNonRoot: true
  runAsUser: 65532
  fsGroup: 65532

securityContext:
  readOnlyRootFilesystem: true
  allowPrivilegeEscalation: false
  capabilities:
    drop: [ALL]
```

The default ports (8000, 9000, 2121) match GoFr's defaults verified in `pkg/gofr/default.go`.

## templates/_helpers.tpl

```yaml
{{- define "gofr-service.name" -}}
{{- default .Chart.Name .Values.nameOverride | trunc 63 | trimSuffix "-" -}}
{{- end -}}

{{- define "gofr-service.fullname" -}}
{{- printf "%s-%s" .Release.Name (include "gofr-service.name" .) | trunc 63 | trimSuffix "-" -}}
{{- end -}}

{{- define "gofr-service.labels" -}}
app.kubernetes.io/name: {{ include "gofr-service.name" . }}
app.kubernetes.io/instance: {{ .Release.Name }}
app.kubernetes.io/managed-by: {{ .Release.Service }}
helm.sh/chart: {{ printf "%s-%s" .Chart.Name .Chart.Version }}
{{- end -}}
```

## templates/deployment.yaml

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: {{ include "gofr-service.fullname" . }}
  labels: {{ include "gofr-service.labels" . | nindent 4 }}
spec:
  replicas: {{ .Values.replicaCount }}
  selector:
    matchLabels:
      app.kubernetes.io/name: {{ include "gofr-service.name" . }}
      app.kubernetes.io/instance: {{ .Release.Name }}
  template:
    metadata:
      labels: {{ include "gofr-service.labels" . | nindent 8 }}
      annotations:
        prometheus.io/scrape: "true"
        prometheus.io/port: "{{ .Values.service.metricsPort }}"
        prometheus.io/path: "/metrics"
    spec:
      securityContext: {{ toYaml .Values.podSecurityContext | nindent 8 }}
      containers:
        - name: app
          image: "{{ .Values.image.repo }}:{{ .Values.image.tag }}"
          imagePullPolicy: {{ .Values.image.pullPolicy }}
          ports:
            - name: http
              containerPort: {{ .Values.service.httpPort }}
            - name: grpc
              containerPort: {{ .Values.service.grpcPort }}
            - name: metrics
              containerPort: {{ .Values.service.metricsPort }}
          env:
            - name: HTTP_PORT
              value: "{{ .Values.service.httpPort }}"
            - name: GRPC_PORT
              value: "{{ .Values.service.grpcPort }}"
            - name: METRICS_PORT
              value: "{{ .Values.service.metricsPort }}"
          {{- range $k, $v := .Values.env }}
            - name: {{ $k }}
              value: {{ $v | quote }}
          {{- end }}
          {{- with .Values.envFromSecrets }}
          envFrom:
            {{- range . }}
            - secretRef:
                name: {{ . }}
            {{- end }}
          {{- end }}
          livenessProbe:
            httpGet:
              path: /.well-known/alive
              port: http
            initialDelaySeconds: 5
            periodSeconds: 10
          readinessProbe:
            httpGet:
              path: /.well-known/health
              port: http
            initialDelaySeconds: 5
            periodSeconds: 10
          resources: {{ toYaml .Values.resources | nindent 12 }}
          securityContext: {{ toYaml .Values.securityContext | nindent 12 }}
      terminationGracePeriodSeconds: 30
```

A few choices worth calling out:

- The probe paths are GoFr's built-in endpoints. `/.well-known/alive` is cheap and exempt from auth by default; `/.well-known/health` includes dependency status and is more truthful for readiness.
- Env vars `HTTP_PORT`, `GRPC_PORT`, `METRICS_PORT` are set explicitly so the container ports and probe ports always agree with what GoFr actually binds.
- The Prometheus scrape annotations point to `metricsPort`. If your platform uses ServiceMonitor/PodMonitor instead, drop the annotations and add a separate template.
- `terminationGracePeriodSeconds: 30` gives GoFr's graceful shutdown time to drain in-flight requests.

## templates/service.yaml

```yaml
apiVersion: v1
kind: Service
metadata:
  name: {{ include "gofr-service.fullname" . }}
  labels: {{ include "gofr-service.labels" . | nindent 4 }}
spec:
  type: {{ .Values.service.type }}
  ports:
    - name: http
      port: {{ .Values.service.httpPort }}
      targetPort: http
    - name: grpc
      port: {{ .Values.service.grpcPort }}
      targetPort: grpc
    - name: metrics
      port: {{ .Values.service.metricsPort }}
      targetPort: metrics
  selector:
    app.kubernetes.io/name: {{ include "gofr-service.name" . }}
    app.kubernetes.io/instance: {{ .Release.Name }}
```

## Optional: Ingress and HPA

Add `templates/ingress.yaml` gated on `.Values.ingress.enabled` and `templates/hpa.yaml` gated on `.Values.autoscaling.enabled`. Keep them off by default so the chart stays simple for first-time users.

## Using the chart

```bash
helm upgrade --install my-api ./chart \
  --set image.tag=$(git rev-parse --short HEAD) \
  --set 'env.LOG_LEVEL=INFO' \
  --wait --timeout 5m
```

Pin the image tag to a Git SHA in production, never `latest`.

## Probes choice

If `/.well-known/health` is slow because it pings databases, you can split:

- **Liveness** → `/.well-known/alive` (process is up)
- **Startup probe** → `/.well-known/health` (deps reachable; tolerate failure during boot)
- **Readiness** → `/.well-known/alive` once startup passes, to avoid removing pods on transient DB blips

Tune per service.

{% faq %}
{% faq-item question="Are these the official GoFr Helm templates?" %}
No. This is reference material to copy into your service repo. A future `gofr-dev/gofr-k8s-starter` repo could host a maintained chart.
{% /faq-item %}
{% faq-item question="Why probe `/.well-known/health` instead of `/.well-known/alive` for readiness?" %}
`/health` includes dependency status, so a pod with a broken DB connection will be removed from service endpoints. `/alive` only confirms the process is running, which is what you want for liveness.
{% /faq-item %}
{% faq-item question="Do I need separate Service ports for HTTP, gRPC, and metrics?" %}
Yes if you want all three reachable. They listen on different ports (8000/9000/2121 by default) and need separate Service entries.
{% /faq-item %}
{% /faq %}
