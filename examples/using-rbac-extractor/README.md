# RBAC Extractor Example

This GoFr example authorizes each request from permissions the application looks up itself, with
`EnableRBACWithExtractor`, instead of from a header or a JWT claim. A permission revoked in the
store is denied on the caller's next request, even though their token has not expired. See the
[RBAC guide](https://gofr.dev/docs/advanced-guide/rbac) for every option.

## How it works

1. `EnableOAuth` verifies the bearer token against your identity provider's JWKS endpoint, with
   `jwt.WithIssuer`, `jwt.WithAudience` and `jwt.WithExpirationRequired`. The extractor trusts the
   token's `sub`, so the token must be checked first.
2. `EnableRBACWithExtractor` loads `configs/rbac.json` and calls the extractor for every request a
   rule protects. The extractor reads `sub` and returns what the store grants that subject now.
   This example keeps the store in memory; a real app queries a database through the container,
   passing `r.Context()`, which carries the extractor's deadline (`RBAC_EXTRACTOR_TIMEOUT`).

```json
{
  "endpoints": [
    { "path": "/status", "methods": ["GET"], "public": true },
    { "path": "/orders", "methods": ["GET"], "requiredPermissions": ["orders:read"] },
    { "path": "/orders", "methods": ["POST"], "requiredPermissions": ["orders:write"] }
  ]
}
```

The store grants `alice` `orders:read` and `orders:write`, and `bob` `orders:read`.

| Token `sub` | `GET /orders` | `POST /orders` | `GET /status` |
|---|---|---|---|
| `alice` | 200 | 201 | 200 |
| `alice`, after `orders:write` is revoked | 200 | 403 | 200 |
| `bob` | 200 | 403 | 200 |
| a subject the store does not know | 403 | 403 | 200 |
| no `sub` | 401 | 401 | 200 |

## Every route needs a rule

A route that no rule matches is served **with no authorization**, and the extractor is never called
for it. `configs/.env` sets `GOFR_RBAC_ROUTE_CHECK=fail`, so an app with a registered route that no
rule covers stops at startup instead of logging an error and serving it open. Keep that setting when
you copy this example, and give a route meant to be open a rule with `"public": true`, like
`/status`.

## Running

Set `JWKS_URL` in `configs/.env` to your identity provider's JWKS endpoint and `TOKEN_ISSUER` to
the `iss` it puts in its tokens, then:

```console
go run main.go
```

`main_test.go` runs the whole app against a local JWKS test server and signs its own tokens, so
`go test ./...` needs no identity provider.
