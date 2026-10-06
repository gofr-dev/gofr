# RBAC Permissions Mode Example

This GoFr example authorizes requests straight from the permissions in a JWT's `scope` claim
(`"orders:read orders:write"`), with no role definitions. See the
[RBAC guide](https://gofr.dev/docs/advanced-guide/rbac) for every option.

## How it works

1. `EnableOAuth` verifies the bearer token against your identity provider's JWKS endpoint and
   puts its claims in the request. It is given `jwt.WithIssuer`, because permissions mode trusts
   what the token says: a token from another issuer, even one signed by a key in the same JWKS,
   must not get in.
2. `EnableRBAC` loads `configs/rbac.json`:

```json
{
  "permissionsClaimPath": "scope",
  "audience": ["orders-api"],
  "endpoints": [
    { "path": "/orders", "methods": ["GET"], "requiredPermissions": ["orders:read"] },
    { "path": "/orders", "methods": ["POST"], "requiredPermissions": ["orders:write"] }
  ]
}
```

Every token below is signed by the identity provider and carries its `iss`, unless the row says
otherwise.

| Token | `GET /orders` | `POST /orders` |
|---|---|---|
| `scope: "orders:read"`, `aud: "orders-api"`, with `exp` | 200 | 403 |
| `scope: "orders:read orders:write"`, `aud: "orders-api"`, with `exp` | 200 | 201 |
| `aud: "billing-api"` (minted for another API) | 401 | 401 |
| no `exp` | 401 | 401 |
| `iss` of another issuer, or no `iss` | 401 | 401 |
| no `scope` | 403 | 403 |

## Running

Set `JWKS_URL` in `configs/.env` to your identity provider's JWKS endpoint and `TOKEN_ISSUER` to
the `iss` it puts in its tokens, then:

```console
go run main.go
```

`main_test.go` runs the whole app against a local JWKS test server and signs its own tokens, so
`go test ./...` needs no identity provider.
