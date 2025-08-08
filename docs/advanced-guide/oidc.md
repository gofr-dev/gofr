
# OpenID Connect (OIDC) Middleware in GoFr

This guide explains how to integrate OpenID Connect (OIDC) authentication support into your GoFr applications using the newly added OIDC middleware and dynamic OIDC discovery helper functions.

---

## Overview

The OIDC middleware contribution provides:

- **Dynamic OIDC Discovery:** Automatically fetch and cache OIDC provider metadata, including JWKS endpoint, issuer, and userinfo endpoint.
- **JWT Validation:** Leverages GoFr’s existing OAuth middleware (`EnableOAuth`) for Bearer token extraction, JWT parsing, signature verification, issuer/audience claim validation, and JWKS key rotation handling.
- **Userinfo Fetch Middleware:** Custom middleware that uses the valid access token to fetch user profile information from the OIDC `userinfo` endpoint and attaches it to the request context.
- **Context Helper:** Convenient function to access user info data in your route handlers.

---

## 1. Fetch OIDC Discovery Metadata

Before enabling OAuth middleware, fetch the provider’s metadata from the OIDC discovery URL (e.g., Google, Okta).

```

meta, err := middleware.FetchOIDCMetadata("https://accounts.google.com/.well-known/openid-configuration")
if err != nil {
// Handle error on startup
}

```

This returns a cached struct with:
- `meta.Issuer`  
- `meta.JWKSURI`  
- `meta.UserInfoEndpoint`  

---

## 2. Enable OAuth Middleware with Discovered Endpoints

Configure GoFr’s built-in OAuth middleware by passing the discovered JWKS URI and issuer:

```

app.EnableOAuth(
meta.JWKSURI,
300, // JWKS refresh interval in seconds (e.g. 5 minutes)
jwt.WithIssuer(meta.Issuer),
// jwt.WithAudience("your-audience") // Optional audience check
)

```

This middleware:
- Extracts and validates Bearer tokens.
- Verifies JWT signature using JWKS keys.
- Caches JWKS and refreshes on rotation or expiry.

---

## 3. Register the OIDC Userinfo Middleware

Register the custom userinfo middleware **after** the OAuth middleware. It calls the OIDC userinfo endpoint with the verified token and attaches the response data to the request context.

```

app.UseMiddleware(middleware.OIDCUserInfoMiddleware(meta.UserInfoEndpoint))

```

---

## 4. Access User Info in Handlers

Within your route handlers, retrieve the fetched user information via the context helper:

```

userInfo, ok := middleware.GetOIDCUserInfo(ctx.Request().Context())
if !ok {
// Handle missing user info (e.g., unauthorized)
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

- **Middleware Order:** Always enable OAuth middleware (`EnableOAuth`) **before** the userinfo middleware so tokens are validated first.
- **Caching:** Discovery metadata and JWKS keys are cached and refreshed automatically to handle key rotation and endpoint changes.
- **Customization:** Use `jwt.ParserOption` to enforce additional claim validation such as audience or custom checks.
- **Error Handling:** Middleware will reject requests with invalid tokens or failed userinfo fetches.
- **Extensibility:** You can extend userinfo middleware to map profile data into your app’s user management as needed.

---

## 6. Summary

| Step                        | Functionality                             |
|-----------------------------|------------------------------------------|
| Fetch discovery metadata    | `FetchOIDCMetadata`                       |
| Enable OAuth validation     | `app.EnableOAuth`                         |
| Fetch and inject user info  | `OIDCUserInfoMiddleware`                   |
| Access user info in handlers| `GetOIDCUserInfo`                         |

---

## 7. Example Integration Snippet

```

meta, err := middleware.FetchOIDCMetadata("https://accounts.google.com/.well-known/openid-configuration")
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

This guide covers how to use your contributed OIDC middleware cleanly and idiomatically within GoFr. For more details, check the middleware source files and tests.

---


