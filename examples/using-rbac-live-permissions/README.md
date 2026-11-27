# RBAC Live Permissions Example

This GoFr example authorizes each request from permissions the application looks up in its own store,
using only released APIs: `EnableOAuth`, an app middleware (`livePermissions`) and `EnableRBAC` in
permissions mode. A permission revoked in the store is denied on the caller's next request, even
though their token has not expired. See the [RBAC guide](https://gofr.dev/docs/advanced-guide/rbac)
for every option.

## How it works

1. `EnableOAuth` verifies the bearer token against your identity provider's JWKS endpoint, with
   `jwt.WithIssuer`, `jwt.WithAudience` and `jwt.WithExpirationRequired`, and puts its claims in the
   request. The middleware trusts the token's `sub`, so the token must be checked first.
2. `livePermissions` reads `sub`, looks up what the store grants that subject now, with a timeout,
   and adds the result to the claims as a `permissions` claim (a `[]any`, which is how RBAC reads
   an array claim). The store's answer replaces any `permissions` claim the token carries itself,
   and a token without a `sub` has that claim removed, so only the store grants access.
3. `EnableRBAC` loads `configs/rbac.json`, whose `"permissionsClaimPath": "permissions"` makes it
   authorize from that claim.

```go
app.EnableOAuth(jwksURL, 10, jwt.WithIssuer(issuer), jwt.WithAudience("orders-api"), jwt.WithExpirationRequired())
app.UseMiddleware(livePermissions(store.Permissions, 2*time.Second)) // after OAuth, before RBAC
app.EnableRBAC()                                                       // permissionsClaimPath: "permissions"
```

```json
{
  "permissionsClaimPath": "permissions",
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
| a subject the store does not know, or no `sub` | 403 | 403 | 200 |
| any subject, while the store is down or slow | 503 | 503 | 200 |

## Order matters

`EnableOAuth`, `UseMiddleware` and `EnableRBAC` all append to one middleware chain, in the order they
are called. Register `livePermissions` **after `EnableOAuth`** (it needs the verified claims) and
**before `EnableRBAC`** (which reads the `permissions` claim when the request passes through it).

If the order is reversed, RBAC runs first and finds no `permissions` claim in the token, so every
protected request gets a 403, even for a caller the store would allow.

## Two things the middleware handles

- **Routes that need no permission check skip the lookup.** `/status` (`"public": true`) and GoFr's
  built-in `/.well-known/` routes (`/.well-known/alive`, `/.well-known/health`) go straight through,
  so a health probe never queries the store, and keeps answering while the store is down. Keep the
  `publicRoutes` table in `main.go` in step with the `"public": true` rules in `configs/rbac.json`.
- **It fails closed.** If the lookup returns an error or does not finish within the timeout, the
  request is answered with a 503 and never reaches RBAC or your handler.

## Every route needs a rule

A route that no rule matches is served **with no authorization**. `configs/.env` sets
`GOFR_RBAC_ROUTE_CHECK=fail`, so an app with a registered route that no rule covers stops at startup
instead of logging an error and serving it open. Keep that setting when you copy this example, and
give a route meant to be open a rule with `"public": true`, like `/status`.

## Running

Set `JWKS_URL` in `configs/.env` to your identity provider's JWKS endpoint and `TOKEN_ISSUER` to
the `iss` it puts in its tokens, then:

```console
go run main.go
```

`main_test.go` runs the whole app against a local JWKS test server and signs its own tokens, so
`go test ./...` needs no identity provider. It checks that revoking a permission in the store turns
the next request with the same token from allowed to 403, that granting it back restores it, that a
store error or timeout gives a 503, and that public and `/.well-known/` routes never call the store.
