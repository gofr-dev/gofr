
# OpenID Connect (OIDC) Middleware in GoFr

This guide shows how to add OpenID Connect (OIDC) authentication to your GoFr applications using the new OIDC middleware and per-provider dynamic discovery with robust caching.

---

## Overview

The OIDC middleware and helpers provide:

- **Dynamic OIDC Discovery:** Per-provider discovery of issuer, JWKS URI, and userinfo endpoint, with robust caching in a struct.
- **JWT Validation:** Out-of-the-box via GoFr’s `EnableOAuth` middleware, including claim checks and JWKS key rotation.
- **Userinfo Fetch Middleware:** Fetches user profile data from the OIDC `userinfo` endpoint, injecting it into the request context.
- **Context Helper:** Retrieves user info easily in handlers.

---

## 1. Set Up OIDC Discovery with Caching (Per-Provider)

Instead of using a single cached global function, create a **DiscoveryCache** for each OIDC provider. You must also use a `context.Context` for timeouts/cancellation.

```

import (
"context"
"time"
"gofr.dev/pkg/gofr/http/middleware"
)

// Create a cache for Google's OIDC provider discovery
cache := middleware.NewDiscoveryCache(
"https://accounts.google.com/.well-known/openid-configuration",
10 * time.Minute, // cache duration
)

// Fetch metadata with context
meta, err := cache.GetMetadata(context.Background())
if err != nil {
// handle error on startup
}

```

This returns per-URL/discovery cached metadata:
- `meta.Issuer`
- `meta.JWKSURI`
- `meta.UserInfoEndpoint`

---

## 2. Enable OAuth Middleware with Discovered Metadata

Apply the discovered JWKS URI and issuer with GoFr's built-in OAuth middleware:

```

app.EnableOAuth(
meta.JWKSURI,
300, // JWKS refresh interval in seconds
jwt.WithIssuer(meta.Issuer),
// jwt.WithAudience("your-audience") // Optional
)

```

- Handles Bearer token extraction, JWT validation, JWKS caching/key rotation.

---

## 3. Register OIDC Userinfo Middleware

After the OAuth middleware, register the userinfo middleware to fetch profile info from the `userinfo` endpoint.

```

app.UseMiddleware(middleware.OIDCUserInfoMiddleware(meta.UserInfoEndpoint))

```

- Userinfo middleware uses the verified Bearer token to call the endpoint and attaches the user info to the request context.

---

## 4. Access User Info in Handlers

Inside your GoFr route handlers, retrieve user info using the helper:

```

userInfo, ok := middleware.GetOIDCUserInfo(ctx.Request().Context())
if !ok {
// Handle missing user info
}
// Use userInfo map for claims like "email", "name", "sub", etc.

```

Example handler returning user info:

```

app.GET("/profile", func(ctx *gofr.Context) (any, error) {
userInfo, ok := middleware.GetOIDCUserInfo(ctx.Request().Context())
if !ok {
return nil, fmt.Errorf("user info not found")
}
return userInfo, nil
})

```

---

## 5. Notes & Best Practices

- **Discovery Cache is per-provider:** Instantiate `DiscoveryCache` for each provider if you work with more than one.
- **Always pass a context:** `GetMetadata` must be called with a valid `context.Context` (e.g., `context.Background()` at startup, request context elsewhere).
- **Middleware order:** Register OAuth middleware before the userinfo middleware.
- **Bearer token extraction:** Follows Go best practices—uses `strings.CutPrefix` and validates non-empty tokens.
- **Documentation and tests:** See codebase and test files (`oidc_test.go`, `discovery_test.go`) for example coverage.

---

## 6. Quick Integration Example

```

import (
"context"
"time"
"gofr.dev/pkg/gofr/http/middleware"
"github.com/golang-jwt/jwt/v5"
)

cache := middleware.NewDiscoveryCache(
"https://accounts.google.com/.well-known/openid-configuration",
10*time.Minute,
)
meta, err := cache.GetMetadata(context.Background())
if err != nil {
log.Fatalf("OIDC discovery failed: %v", err)
}

app.EnableOAuth(
meta.JWKSURI,
300,
jwt.WithIssuer(meta.Issuer),
)

app.UseMiddleware(middleware.OIDCUserInfoMiddleware(meta.UserInfoEndpoint))

app.GET("/profile", func(ctx *gofr.Context) (any, error) {
userInfo, ok := middleware.GetOIDCUserInfo(ctx.Request().Context())
if !ok {
return nil, fmt.Errorf("user info not found")
}
return userInfo, nil
})

```

---

## 7. Summary Table

| Component                | Use                                                                              |
|--------------------------|-----------------------------------------------------------------------------------|
| DiscoveryCache           | Per-provider discovery and caching (thread-safe/isolated)                         |
| GetMetadata (with ctx)   | Fetch/cached OIDC metadata                                                       |
| EnableOAuth              | Configure JWT validation and key management                                      |
| OIDCUserInfoMiddleware   | Fetch and inject user profile info                                               |
| GetOIDCUserInfo          | Access user info in handlers                                                     |

---

This guide covers how to use your contributed OIDC middleware cleanly and idiomatically within GoFr. For more details, check the middleware source files and tests.

---

