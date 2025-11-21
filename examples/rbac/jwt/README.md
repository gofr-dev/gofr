# JWT RBAC Example

## Overview

This example demonstrates **JWT-based role-based access control (RBAC)**. Roles are extracted from JWT tokens that are validated using OAuth/JWKS endpoints.

## Use Case

**When to use:**
- Public-facing APIs requiring secure authentication
- Microservices with JWT-based authentication
- Applications integrated with OAuth2/OIDC providers
- Multi-service architectures with centralized authentication

**Not suitable for:**
- Simple internal APIs (use header-based RBAC)
- Applications without JWT infrastructure
- Legacy systems without OAuth support

## How It Works

1. **JWT Validation**: OAuth middleware validates JWT tokens using JWKS endpoint
2. **Role Extraction**: Role is extracted from JWT claims (e.g., `"role"` claim)
3. **Route Matching**: Routes are matched against patterns in the config file
4. **Authorization**: The extracted role is checked against allowed roles
5. **Audit Logging**: All authorization decisions are automatically logged

## Configuration

### RBAC Config (`configs/rbac.json`)

```json
{
  "route": {
    "/api/users": ["admin", "editor", "viewer"],
    "/api/admin/*": ["admin"]
  },
  "overrides": {
    "/health": true
  }
}
```

### JWT Token Structure

The JWT token should contain a role claim. Example:

```json
{
  "sub": "user123",
  "role": "admin",
  "iat": 1234567890,
  "exp": 1234571490
}
```

**JWT Role Claim Parameter (`roleClaim`):**

The `roleClaim` parameter in `app.EnableRBACWithJWT()` or `rbac.NewJWTRoleExtractor()` specifies the path to the role in JWT claims. It supports multiple formats:

| Format | Example | JWT Claim Structure |
|--------|---------|---------------------|
| **Simple Key** | `"role"` | `{"role": "admin"}` |
| **Array Notation** | `"roles[0]"` | `{"roles": ["admin", "user"]}` - extracts first element |
| **Array Notation** | `"roles[1]"` | `{"roles": ["admin", "user"]}` - extracts second element |
| **Dot Notation** | `"permissions.role"` | `{"permissions": {"role": "admin"}}` |
| **Deeply Nested** | `"user.permissions.role"` | `{"user": {"permissions": {"role": "admin"}}}` |

**Notes:**
- If `roleClaim` is empty (`""`), it defaults to `"role"`
- The extracted value is converted to string automatically
- Array indices must be valid integers (e.g., `[0]`, `[1]`, not `[invalid]`)
- Array indices must be within bounds (e.g., `roles[5]` fails if array has only 2 elements)

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
curl -H "Authorization: Bearer $TOKEN" http://localhost:8000/api/admin
```

### 4. Testing with Mock Server (for development)

For testing, you can use the included `mock_jwks_server.go`:

```go
// In your test file
mockJWKS, _ := NewMockJWKSServer()
defer mockJWKS.Close()

app.EnableOAuth(mockJWKS.JWKSEndpoint(), 10)
```

## API Endpoints

- `GET /api/users` - Accessible by: admin, editor, viewer
- `GET /api/admin` - Accessible by: admin only

## Features Demonstrated

1. **JWT Validation**: Automatic token validation via OAuth middleware
2. **Flexible Role Claims**: Support for various JWT claim structures
3. **Route-Based Authorization**: Pattern matching for routes
4. **Secure by Default**: Tokens are cryptographically validated

## JWT Claim Path Examples

### Simple Claim
```json
{"role": "admin", "sub": "user123"}
```
```go
app.EnableRBACWithJWT("configs/rbac.json", "role")
```

### Array Claim (First Element)
```json
{"roles": ["admin", "user"], "sub": "user123"}
```
```go
app.EnableRBACWithJWT("configs/rbac.json", "roles[0]")  // Extracts "admin"
```

### Array Claim (Second Element)
```json
{"roles": ["admin", "user"], "sub": "user123"}
```
```go
app.EnableRBACWithJWT("configs/rbac.json", "roles[1]")  // Extracts "user"
```

### Nested Claim
```json
{
  "permissions": {
    "role": "admin",
    "scope": "read:write"
  },
  "sub": "user123"
}
```
```go
app.EnableRBACWithJWT("configs/rbac.json", "permissions.role")
```

### Deeply Nested Claim
```json
{
  "user": {
    "permissions": {
      "role": "admin"
    }
  },
  "sub": "user123"
}
```
```go
app.EnableRBACWithJWT("configs/rbac.json", "user.permissions.role")
```

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

## Integration with OAuth Providers

### Auth0
```go
app.EnableOAuth("https://your-domain.auth0.com/.well-known/jwks.json", 10)
```

### Keycloak
```go
app.EnableOAuth("https://keycloak.example.com/realms/your-realm/protocol/openid-connect/certs", 10)
```

### AWS Cognito
```go
app.EnableOAuth("https://cognito-idp.region.amazonaws.com/userPoolId/.well-known/jwks.json", 10)
```

