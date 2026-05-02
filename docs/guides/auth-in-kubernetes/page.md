---
description: "Run GoFr authentication in Kubernetes: OAuth/JWT with JWKS discovery, Vault-backed secrets, and choosing between API key, mTLS, and JWT for service-to-service."
nextjs:
  metadata:
    title: "GoFr Auth in Kubernetes: JWT, JWKS, Vault, and mTLS"
    description: "Run GoFr authentication in Kubernetes: OAuth/JWT with JWKS discovery, Vault-backed secrets, and choosing between API key, mTLS, and JWT for service-to-service."
---

# Auth in Kubernetes

{% answer %}
GoFr supports Basic Auth, API key auth, and OAuth 2.0 JWT validation against a JWKS endpoint (`EnableOAuth(jwksEndpoint, refreshInterval, options...)`). In Kubernetes, point the JWKS URL at your IdP (cluster-internal Service or public URL), inject API keys via Vault Agent or sealed Secrets, and prefer mTLS or JWT over static keys for service-to-service calls.
{% /answer %}

## What GoFr provides

Three authentication methods are exposed on the App, all verified in `pkg/gofr/auth.go`:

- `EnableBasicAuth(credentials...)` — pairs of username/password.
- `EnableBasicAuthWithValidator(fn)` — custom validator with access to the container.
- `EnableAPIKeyAuth(keys...)` — `X-Api-Key` header check.
- `EnableAPIKeyAuthWithValidator(fn)` — custom validator.
- `EnableOAuth(jwksEndpoint, refreshIntervalSeconds, options ...jwt.ParserOption)` — JWT validation with periodic JWKS refresh.

A single call enables auth on both HTTP and gRPC. `/.well-known/alive` is exempted by default; `/.well-known/health` is also exempted by default but should normally be re-protected.

For full code examples, see [Authentication](/docs/advanced-guide/authentication).

## OAuth 2.0 with JWKS in Kubernetes

`EnableOAuth` registers an internal HTTP service named `gofr_oauth` to fetch keys, then validates JWTs on every request. Two deployment patterns:

### Public IdP (Auth0, Okta, Google, Azure AD)

```go
app.EnableOAuth("https://your-tenant.auth0.com/.well-known/jwks.json", 3600,
    jwt.WithAudience("https://api.example.com"),
    jwt.WithIssuer("https://your-tenant.auth0.com/"),
    jwt.WithExpirationRequired())
```

Egress from your cluster must be allowed to reach the IdP. If you have a strict NetworkPolicy, allowlist the IdP CIDR or use a forward proxy.

### Cluster-internal IdP (Keycloak, Dex, Hydra)

If your IdP runs in the same cluster, point at its in-cluster Service DNS:

```go
app.EnableOAuth("http://keycloak.iam.svc.cluster.local:8080/realms/prod/protocol/openid-connect/certs", 3600)
```

The JWKS fetch is cheap, and the `refreshInterval` controls how stale your key cache can be. A typical value is 600–3600 seconds. After key rotation by the IdP, requests with old tokens fail until the cache refreshes.

## Storing API keys: Vault

`EnableAPIKeyAuth` takes the keys directly. In production, source them from a secret manager:

- **Vault Agent sidecar** — mounts a rendered file or sets env vars at pod start, refreshing on a schedule. Inject via the `vault.hashicorp.com/agent-inject: "true"` annotation and read with `app.Config.Get`.
- **External Secrets Operator** — syncs Vault, AWS Secrets Manager, or GCP Secret Manager into a Kubernetes Secret. Mount as env vars.
- **Sealed Secrets** — fine for low-rotation keys committed to GitOps repos.

Avoid bundling API keys into ConfigMaps or container images.

## Service-to-service auth: pick one model

For internal calls between GoFr services, three reasonable models:

1. **mTLS via service mesh** — Istio `PeerAuthentication: STRICT` or Linkerd automatic mTLS. No GoFr code change needed. Strongest identity, requires mesh ops.
2. **JWT** — the calling service obtains a token (client credentials or workload identity) and the receiving GoFr service uses `EnableOAuth`. Works without a mesh.
3. **Shared API key** — simple but rotation-heavy and gives no per-caller identity. Acceptable for low-trust internal endpoints.

Avoid mixing on a single endpoint. Pick one per trust boundary.

## Refresh strategy

For OAuth, GoFr refreshes JWKS on the interval you pass. The receiving service does not refresh user tokens — that is the client's responsibility. For long-lived clients (cron jobs, batch workers), refresh ahead of expiry rather than on 401.

## Accessing claims in handlers

Once OAuth is enabled, `ctx.GetAuthInfo().GetClaims()` returns the parsed claim map. Cast specific claims as needed:

```go
claims := ctx.GetAuthInfo().GetClaims()
userID, _ := claims["sub"].(string)
```

## Liveness must stay open

Kubernetes liveness probes fire from the kubelet without credentials. GoFr exempts `/.well-known/alive` from auth so probes succeed. Do not put auth in front of the alive endpoint via an Ingress filter.

## Health endpoint

`/.well-known/health` is exempted by default but reveals dependency status. In production, re-enable auth on it (or restrict via NetworkPolicy to your monitoring namespace) so it is not enumerable from the public internet.

## TLS

Always serve credentials and tokens over TLS. Inside the cluster, the mesh (or `CERT_FILE` / `KEY_FILE` configured directly on GoFr) terminates TLS; on the edge, the Ingress does.

{% faq %}
{% faq-item question="Can I run JWKS-based JWT auth without a service mesh?" %}
Yes. `EnableOAuth` just needs an HTTP-reachable JWKS endpoint. The mesh is optional and adds mTLS at the network layer.
{% /faq-item %}
{% faq-item question="Where should API keys be stored?" %}
In a secret manager (Vault, AWS/GCP Secrets Manager) and surfaced to the pod as environment variables via Vault Agent or External Secrets Operator. Never in ConfigMaps or container images.
{% /faq-item %}
{% faq-item question="Does enabling auth in GoFr also protect gRPC?" %}
Yes. A single call to `EnableBasicAuth`, `EnableAPIKeyAuth`, or `EnableOAuth` registers middleware on both the HTTP and gRPC servers — verified in `pkg/gofr/auth.go`.
{% /faq-item %}
{% /faq %}
