# Permission-Based RBAC (JWT) Example

## Overview

This example demonstrates **permission-based access control** with **JWT-based role extraction**. It combines the security of JWT tokens with the flexibility of permission-based authorization.

## Use Case

**When to use:**
- Public-facing APIs requiring secure authentication
- Fine-grained permission control needed
- JWT tokens already in use for authentication
- Multi-service architectures with centralized auth

**Example scenarios:**
- SaaS platforms with subscription tiers
- Enterprise APIs with complex permission requirements
- Microservices with JWT-based service-to-service auth

## How It Works

1. **JWT Validation**: OAuth middleware validates JWT tokens
2. **Role Extraction**: Role extracted from JWT claims (e.g., `"role"` claim)
3. **Permission Mapping**: Route + HTTP method mapped to required permission
4. **Permission Check**: System checks if user's role has the required permission
5. **Authorization**: Access granted/denied based on permission check

## Configuration

### RBAC Config (`configs/rbac.json`)

```json
{
  "route": {
    "/api/*": ["admin", "editor"]
  },
  "enablePermissions": true
}
```

### Permission Configuration (in code)

```go
config.PermissionConfig = &rbac.PermissionConfig{
    Permissions: map[string][]string{
        "users:read":   {"admin", "editor", "viewer"},
        "users:write":  {"admin", "editor"},
        "users:delete": {"admin"},
    },
    RoutePermissionMap: map[string]string{
        "GET /api/users":    "users:read",
        "POST /api/users":   "users:write",
        "DELETE /api/users": "users:delete",
    },
}
```

### JWT Token Structure

The JWT token should contain a role claim:

```json
{
  "sub": "user123",
  "role": "admin",
  "iat": 1234567890,
  "exp": 1234571490
}
```

## Setup Instructions

### 1. Configure JWKS Endpoint

Update the JWKS endpoint in `main.go`:

```go
app.EnableOAuth("https://your-auth-server.com/.well-known/jwks.json", 10)
```

### 2. Start the Application

```bash
go run main.go
```

### 3. Test with JWT Tokens

```bash
# Get a JWT token from your OAuth provider
TOKEN="your-jwt-token-here"

# Test endpoints
curl -H "Authorization: Bearer $TOKEN" http://localhost:8000/api/users
curl -X POST -H "Authorization: Bearer $TOKEN" http://localhost:8000/api/users
curl -X DELETE -H "Authorization: Bearer $TOKEN" http://localhost:8000/api/users
```

## API Endpoints

- `GET /api/users` - Requires: `users:read` permission
- `POST /api/users` - Requires: `users:write` permission
- `DELETE /api/users` - Requires: `users:delete` permission
- `GET /api/posts` - Requires: `posts:read` permission
- `POST /api/posts` - Requires: `posts:write` permission

## Permission Matrix

| Role   | users:read | users:write | users:delete | posts:read | posts:write |
|--------|------------|-------------|--------------|-----------|------------|
| admin  | ✅         | ✅          | ✅           | ✅        | ✅         |
| editor | ✅         | ✅          | ❌           | ❌        | ❌         |
| viewer | ✅         | ❌          | ❌           | ✅        | ❌         |
| author | ❌         | ❌          | ❌           | ✅        | ✅         |

## Features Demonstrated

1. **JWT + Permissions**: Secure token validation with fine-grained permissions
2. **Flexible Role Claims**: Support for various JWT claim structures
3. **Action-Level Control**: Different permissions for different HTTP methods
4. **Production Ready**: Secure and scalable for enterprise use

## Advantages

✅ **Secure**: JWT tokens are cryptographically validated  
✅ **Flexible**: Permissions can be assigned to multiple roles  
✅ **Scalable**: Easy to add new permissions  
✅ **Standard**: Uses OAuth2/OIDC standards

## Security Considerations

✅ **Secure for production** when:
- JWKS endpoint is properly secured (HTTPS)
- JWT tokens are signed with RS256/RS512
- Token expiration is properly validated
- Issuer and audience claims are validated

⚠️ **Ensure:**
- JWKS endpoint is accessible from your application
- Refresh interval is appropriate for your use case
- OAuth provider is trusted and secure

