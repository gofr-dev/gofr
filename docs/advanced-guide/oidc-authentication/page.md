# OIDC Authentication

OpenID Connect (OIDC) is an identity layer built on top of OAuth 2.0 that enables secure user authentication and transmission of user profile information. It allows clients to verify end-user identities based on authentication performed by an authorization server.

## Overview

Authentication is a critical part of securing web applications by ensuring only authorized users can access protected resources. GoFR supports OIDC integration through middleware that validates Bearer tokens and fetches user information from the OIDC provider.

## Setup

To enable OIDC authentication in GoFR, configure the middleware with your OIDC provider’s UserInfo endpoint. This endpoint is used to validate access tokens and retrieve user claims.

## Usage

Here is an example of enabling OIDC authentication middleware in a GoFR application:

```go
package main

import (
"gofr.dev/gofr/pkg/gofr"
"gofr.dev/gofr/pkg/gofr/http/middleware"
)

func main() {
app := gofr.New()

// Configure OIDC Auth Provider with your UserInfo endpoint
oidcProvider := &middleware.OIDCAuthProvider{
    UserInfoEndpoint: "https://your-oidc-provider.com/userinfo",
}

// Use the OIDC middleware for authentication
app.Use(middleware.AuthMiddleware(oidcProvider))

// Define a protected route
app.GET("/profile", func(c *gofr.Context) (any, error) {
    userClaims := c.UserInfo() // Access claims set by the middleware
    return userClaims, nil
})

app.Run()
}
```

## Error Handling

The middleware handles common error scenarios including:

- Missing or empty Bearer tokens
- Invalid or expired tokens
- Failure to fetch or parse user info from the UserInfo endpoint

Appropriate HTTP 401 (Unauthorized) responses will be returned by the middleware in these cases.

## Tips

- Configure reasonable HTTP client timeouts in the middleware to avoid delays calling the UserInfo endpoint.
- Consider caching user info responses if your application makes frequent authorization checks to improve performance.
- Test your OIDC integration using tokens issued by your authorization server and confirm user claims are correctly propagated.

---

This integration enables robust and standardized authentication flows in GoFR applications using OpenID Connect.
