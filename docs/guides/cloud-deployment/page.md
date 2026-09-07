---
description: "Deploy GoFr on AWS EKS, GCP GKE, and Azure AKS: provider Ingress controllers, LoadBalancer quirks, managed databases, and IAM-based credentials."
nextjs:
  metadata:
    title: "GoFr on AWS EKS, GCP GKE & Azure AKS: Cloud Deployment"
    description: "Deploy GoFr on AWS EKS, GCP GKE, and Azure AKS: provider Ingress controllers, LoadBalancer quirks, managed databases, and IAM-based credentials."
---

# Cloud Deployment

{% answer %}
A GoFr container is a stock Linux Go binary listening on `HTTP_PORT` (default 8000), `GRPC_PORT` (default 9000), and `METRICS_PORT` (default 2121). It runs unchanged on EKS, GKE, and AKS — what differs is the Ingress controller, the LoadBalancer flavor, and how the pod gets credentials for managed datasources.
{% /answer %}

## Common Kubernetes shape

Regardless of cloud, your pod exposes:

- `8000` — HTTP API (overridable via `HTTP_PORT`)
- `9000` — gRPC (overridable via `GRPC_PORT`)
- `2121` — Prometheus metrics (overridable via `METRICS_PORT`; set to `0` to disable)
- `/.well-known/alive` — liveness
- `/.well-known/health` — readiness (covers dependencies)

These defaults are confirmed in `pkg/gofr/default.go` and `pkg/gofr/factory.go`.

## AWS EKS

**Cluster bring-up.** Minimal `eksctl` invocation:

```bash
eksctl create cluster \
  --name <name> \
  --region <region> \
  --nodegroup-name <ng> \
  --node-type t3.medium \
  --nodes 2 \
  --managed
```

**Ingress controller install.** Install the AWS Load Balancer Controller via Helm (after IRSA is set up for the controller's ServiceAccount — see the controller's install docs for the IAM policy and ServiceAccount creation):

```bash
helm install aws-load-balancer-controller eks/aws-load-balancer-controller \
  -n kube-system \
  --set clusterName=<name> \
  --set serviceAccount.create=false \
  --set serviceAccount.name=aws-load-balancer-controller
```

**Ingress.** Use the AWS Load Balancer Controller, which provisions an ALB from an `Ingress` resource:

```yaml
apiVersion: networking.k8s.io/v1
kind: Ingress
metadata:
  name: gofr-api
  annotations:
    alb.ingress.kubernetes.io/scheme: internet-facing
    alb.ingress.kubernetes.io/target-type: ip
    alb.ingress.kubernetes.io/healthcheck-path: /.well-known/alive
spec:
  ingressClassName: alb
  rules: [...]
```

For raw TCP (e.g., gRPC) prefer a `Service: type=LoadBalancer` annotated with `service.beta.kubernetes.io/aws-load-balancer-type: nlb`. ALB is L7-only.

**IAM.** Use IRSA (IAM Roles for Service Accounts). Annotate the ServiceAccount with `eks.amazonaws.com/role-arn: arn:aws:iam::...`. The AWS SDK inside any GoFr S3 / SQS / SNS datasource will pick the credentials up automatically. Do not bake static keys into env vars.

```yaml
apiVersion: v1
kind: ServiceAccount
metadata:
  name: orders
  namespace: default
  annotations:
    eks.amazonaws.com/role-arn: arn:aws:iam::<account-id>:role/<role-name>
```

**Datasources.** RDS for PostgreSQL/MySQL works directly with GoFr's SQL driver — set `DB_HOST`, `DB_PORT`, etc. ElastiCache Redis works via the Redis datasource. Aurora's failover is handled by the cluster endpoint; GoFr will reconnect on failure.

**Persistent storage.** Only relevant if you use the `local` file storage driver. Use an EBS-backed `PersistentVolumeClaim`. For multi-AZ, switch to S3 file storage instead.

**Smoke test.**

```bash
# Apply the manifests from /docs/guides/deploying-to-kubernetes
kubectl apply -f k8s/
kubectl wait --for=condition=available --timeout=120s deployment/orders
kubectl port-forward svc/orders 8080:80 &
curl -s http://localhost:8080/.well-known/health
```

**Cost note.** A 2-node `t3.medium` managed node group is the smallest commonly-used EKS shape; you also pay the EKS control plane hourly fee per cluster. Expect a low-double-digit USD/day for an idle cluster at list price, before egress, NAT Gateway, EBS, ALB hours, and CloudWatch ingestion. Use the [AWS Pricing Calculator](https://calculator.aws/) for an account-accurate figure.

For canonical syntax see: `https://kubernetes-sigs.github.io/aws-load-balancer-controller/`.

## GCP GKE

**Cluster bring-up.** Minimal `gcloud` invocation:

```bash
gcloud container clusters create <name> \
  --region <region> \
  --num-nodes 2 \
  --machine-type e2-standard-2 \
  --release-channel regular
```

**Ingress controller install.** GKE ships the GCE Ingress controller enabled by default — no install step needed. If you'd rather use NGINX or another controller, install it the same way you would on any cluster.

**Ingress.** GKE has a built-in GCE Ingress controller that provisions an HTTP(S) Load Balancer:

```yaml
apiVersion: networking.k8s.io/v1
kind: Ingress
metadata:
  name: gofr-api
spec:
  ingressClassName: gce
  rules: [...]
```

For container-native load balancing (recommended), expose the Service as `type: ClusterIP` with the `cloud.google.com/neg: '{"ingress": true}'` annotation.

**LoadBalancer tier.** A `Service: type=LoadBalancer` defaults to the Premium network tier. To force Standard, set `cloud.google.com/network-tier: Standard`. Premium gives lower latency but higher cost.

**IAM.** Use Workload Identity. Bind a Kubernetes ServiceAccount to a Google IAM service account with `iam.gke.io/gcp-service-account` annotation. GoFr's GCS file storage and Pub/Sub datasources will use those credentials.

```yaml
apiVersion: v1
kind: ServiceAccount
metadata:
  name: orders
  namespace: default
  annotations:
    iam.gke.io/gcp-service-account: <gsa-name>@<project-id>.iam.gserviceaccount.com
```

**Datasources.** Cloud SQL: connect via the Cloud SQL Auth Proxy as a sidecar, or use Private IP and a VPC-native cluster. For Memorystore Redis, set `REDIS_HOST` to the private IP.

**Persistent storage.** `pd-ssd` PersistentDisk works for the local file storage driver. PD is zonal — use Regional PD for multi-zone availability.

**Smoke test.**

```bash
# Apply the manifests from /docs/guides/deploying-to-kubernetes
kubectl apply -f k8s/
kubectl wait --for=condition=available --timeout=120s deployment/orders
kubectl port-forward svc/orders 8080:80 &
curl -s http://localhost:8080/.well-known/health
```

**Cost note.** Two `e2-standard-2` nodes is a common minimal shape; GKE Standard also charges a per-cluster management fee (Autopilot bills differently — per-pod). Idle list-price cost is typically a low-double-digit USD/day before egress, NAT, persistent disks, LB forwarding rules, and Cloud Logging ingestion. The [Google Cloud Pricing Calculator](https://cloud.google.com/products/calculator) gives an account-accurate figure.

Canonical docs: `https://cloud.google.com/kubernetes-engine/docs/concepts/ingress`.

## Azure AKS

**Cluster bring-up.** Minimal `az` invocation:

```bash
az aks create \
  --resource-group <rg> \
  --name <name> \
  --node-count 2 \
  --node-vm-size Standard_DS2_v2 \
  --enable-managed-identity \
  --generate-ssh-keys
```

**Ingress controller install.** Either install NGINX via Helm:

```bash
helm install ingress-nginx ingress-nginx/ingress-nginx \
  -n ingress-nginx \
  --create-namespace
```

or enable the Application Gateway Ingress Controller (AGIC) for an Azure-native L7 path — see Microsoft's AGIC install docs for the exact flags, which depend on whether you bring your own Application Gateway or let AKS create one.

**Ingress.** Two common options:

- Application Gateway Ingress Controller (AGIC) — uses an Azure Application Gateway, integrates with WAF.
- NGINX Ingress Controller — vendor-neutral, runs anywhere.

AGIC sample (modern form): set `spec.ingressClassName: azure-application-gateway` on the Ingress instead of using the deprecated `kubernetes.io/ingress.class` annotation.

**LoadBalancer.** A `Service: type=LoadBalancer` provisions an Azure Standard Load Balancer by default. For internal-only, add `service.beta.kubernetes.io/azure-load-balancer-internal: "true"`.

**IAM.** Use AKS Managed Identity with workload federation. Bind a UserAssignedIdentity via federated credentials. The Azure SDK in GoFr's Azure file storage and Event Hub datasources picks them up automatically.

```yaml
apiVersion: v1
kind: ServiceAccount
metadata:
  name: orders
  namespace: default
  annotations:
    azure.workload.identity/client-id: <client-id>
```

**Datasources.** Azure Database for PostgreSQL/MySQL works with GoFr's SQL driver over private endpoints. Azure Cache for Redis works via the Redis datasource.

**Persistent storage.** Azure Disk (`managed-csi`) for single-zone; Azure Files (CSI) when you need ReadWriteMany.

**Smoke test.**

```bash
# Apply the manifests from /docs/guides/deploying-to-kubernetes
kubectl apply -f k8s/
kubectl wait --for=condition=available --timeout=120s deployment/orders
kubectl port-forward svc/orders 8080:80 &
curl -s http://localhost:8080/.well-known/health
```

**Cost note.** Two `Standard_DS2_v2` nodes is a common minimal AKS shape; the AKS control plane is free in the Free tier (paid in Standard/Premium tiers, which add an SLA fee per cluster-hour). Expect a low-double-digit USD/day at list price for an idle cluster before egress, public IPs, managed disks, Application Gateway hours, and Log Analytics ingestion. Use the [Azure Pricing Calculator](https://azure.microsoft.com/pricing/calculator/) for an account-accurate figure.

Canonical docs: `https://learn.microsoft.com/azure/aks/`.

## Common gotchas

- Set `terminationGracePeriodSeconds` longer than your slowest in-flight request so GoFr's graceful shutdown can drain.
- If you front gRPC with an L7 LB (ALB, GCE), confirm HTTP/2 end-to-end — ALB needs `BackendProtocolVersion=GRPC`.
- Use cloud-native logging (CloudWatch / Cloud Logging / Azure Monitor) only after confirming GoFr's structured JSON logs are not double-parsed.

{% faq %}
{% faq-item question="Do I need cloud-specific code for GoFr to run on EKS, GKE, or AKS?" %}
No. GoFr runs the same binary everywhere. Cloud differences live in the Kubernetes manifests (Ingress, IAM bindings) and in connection strings for managed datasources.
{% /faq-item %}
{% faq-item question="Should I use static cloud credentials in environment variables?" %}
No. Use IRSA on EKS, Workload Identity on GKE, and Managed Identity on AKS. The cloud SDKs that GoFr's datasources sit on top of will pick up the credentials automatically.
{% /faq-item %}
{% faq-item question="Which Ingress controller is required for GoFr?" %}
None — GoFr does not depend on a specific Ingress. ALB (EKS), GCE (GKE), AGIC or NGINX (AKS) all work as long as they can route to port 8000.
{% /faq-item %}
{% /faq %}
