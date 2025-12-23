# Role-Based Access Control (RBAC) in GoFr

Role-Based Access Control (RBAC) is a security mechanism that restricts access to resources based on user roles and permissions. GoFr provides a pure config-based RBAC middleware that supports multiple authentication methods, fine-grained permissions, and role inheritance.

## Overview

- ✅ **Pure Config-Based** - All authorization rules in JSON/YAML files
- ✅ **Two-Level Authorization Model** - Roles define permissions, endpoints require permissions (no direct role-to-route mapping)
- ✅ **Multiple Auth Methods** - Header-based and JWT-based role extraction
- ✅ **Permission-Based** - Fine-grained permissions 
- ✅ **Role Inheritance** - Roles inherit permissions from other roles

## Quick Start

```go
package main

import (
	"gofr.dev/pkg/gofr"
)

func main() {
	app := gofr.New()
	
	// Use default paths (configs/rbac.json, configs/rbac.yaml, configs/rbac.yml)
	// Tries configs/rbac.json, then configs/rbac.yaml, then configs/rbac.yml
	app.EnableRBAC()
	
	// Or with custom config path
	app.EnableRBAC("configs/custom-rbac.json")
	
	app.GET("/api/users", handler)
	app.Run()
}
```

**Configuration** (`configs/rbac.json`):

```json
{
  "roleHeader": "X-User-Role",
  "roles": [
    {
      "name": "admin",
      "permissions": ["users:read", "users:write", "users:delete", "posts:read", "posts:write"]
    },
    {
      "name": "editor",
      "permissions": ["users:write", "posts:write"],
      "inheritsFrom": ["viewer"]
    },
    {
      "name": "viewer",
      "permissions": ["users:read", "posts:read"]
    }
  ],
  "endpoints": [
    {
      "path": "/health",
      "methods": ["GET"],
      "public": true
    },
    {
      "path": "/api/users",
      "methods": ["GET"],
      "requiredPermissions": ["users:read"]
    },
    {
      "path": "/api/users",
      "methods": ["POST"],
      "requiredPermissions": ["users:write"]
    }
  ]
}
```

> **💡 Best Practice**: For production/public APIs, use JWT-based RBAC instead of header-based RBAC for better security.

## Configuration

### Role Extraction

**Header-Based** (for internal/trusted networks):
```json
{
  "roleHeader": "X-User-Role"
}
```

**JWT-Based** (for production/public APIs):
```json
{
  "jwtClaimPath": "role"  // or "roles[0]", "permissions.role", etc.
}
```

**Precedence**: If both are set, **only JWT is considered**. The header is not checked when `jwtClaimPath` is configured, even if JWT extraction fails.

**JWT Claim Path Formats**:
- `"role"` → `{"role": "admin"}`
- `"roles[0]"` → `{"roles": ["admin", "user"]}` (first element)
- `"permissions.role"` → `{"permissions": {"role": "admin"}}`

### Roles and Permissions

```json
{
  "roles": [
    {
      "name": "admin",
      "permissions": ["users:read", "users:write", "users:delete", "posts:read", "posts:write"]  // Explicit permissions (wildcards not supported)
    },
    {
      "name": "editor",
      "permissions": ["users:write", "posts:write"],  // Only additional permissions
      "inheritsFrom": ["viewer"]  // Inherits viewer's permissions
    },
    {
      "name": "viewer",
      "permissions": ["users:read", "posts:read"]
    }
  ]
}
```

**Note**: When using `inheritsFrom`, only specify additional permissions - inherited ones are automatically included.

### Endpoint Mapping

```json
{
  "endpoints": [
    {
      "path": "/health",
      "methods": ["GET"],
      "public": true  // Bypasses authorization
    },
    {
      "path": "/api/users",
      "methods": ["GET"],
      "requiredPermissions": ["users:read"]
    },
    {
      "path": "^/api/users/\\d+$",  // Regex pattern for strict validation (numeric IDs only) - automatically detected
      "methods": ["DELETE"],
      "requiredPermissions": ["users:delete"]
    },
    {
      "path": "/api/admin/*",  // Wildcard pattern - matches all sub-paths
      "methods": ["*"],  // All methods
      "requiredPermissions": ["admin:read", "admin:write"]  // Multiple permissions (OR logic)
    }
  ]
}
```

**Route Patterns**:
- **Exact**: `"/api/users"` matches exactly `/api/users`
- **Wildcard**: `"/api/*"` matches `/api/users`, `/api/posts`, etc.
- **Regex**: `"^/api/users/\\d+$"` matches `/api/users/123`, etc.

### Path Parameters

For endpoints with path parameters (e.g., `/api/users/{id}`), the `{id}` syntax in the `path` field is **documentation only** and does not work for matching. You must use either a **wildcard** or **regex** pattern.

**Use Wildcard (`/*`) for Simple Cases**:
```json
{
  "path": "/api/users/*",
  "methods": ["DELETE"],
  "requiredPermissions": ["users:delete"]
}
```
- ✅ Matches: `/api/users/123`, `/api/users/abc`, `/api/users/anything`
- Simple and flexible, but accepts any value

**Use Regex for Strict Validation**:
```json
{
  "path": "^/api/users/\\d+$",  // Regex pattern - automatically detected when starts with ^ or contains regex special chars
  "methods": ["DELETE"],
  "requiredPermissions": ["users:delete"]
}
```
- ✅ Matches: `/api/users/123`, `/api/users/456`
- ❌ Does not match: `/api/users/abc`, `/api/users/123abc`
- Provides strict validation (numeric IDs, UUIDs, etc.)
- Regex patterns are automatically detected and precompiled for better performance

**Common Regex Patterns**:
- Numeric IDs: `"^/api/users/\\d+$"` (matches `/api/users/123`)
- UUIDs: `"^/api/users/[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$"` (matches `/api/users/550e8400-e29b-41d4-a716-446655440000`)
- Alphanumeric: `"^/api/users/[a-zA-Z0-9]+$"` (matches `/api/users/user123`)

**Note**: Regex patterns are automatically detected when the `path` field starts with `^`, ends with `$`, or contains regex special characters like `\d`, `\w`, `[`, `(`, `?`.

## JWT-Based RBAC

For production/public APIs, use JWT-based role extraction:

```go
app := gofr.New()

// Enable OAuth middleware first (required for JWT validation)
app.EnableOAuth("https://auth.example.com/.well-known/jwks.json", 10)

// Enable RBAC with config path (or use app.EnableRBAC() for default paths)
app.EnableRBAC("configs/rbac.json")
```

**Configuration** (`configs/rbac.json`):

```json
{
  "jwtClaimPath": "role",  // or "roles[0]", "permissions.role", etc.
  "roles": [...],
  "endpoints": [...]
}
```

## Accessing Role in Handlers

For business logic, you can access the user's role from the request context:

**JWT-Based RBAC** (when using JWT role extraction):

```go
func handler(ctx *gofr.Context) (interface{}, error) {
	// Get JWT claims from context
	claims := ctx.GetAuthInfo().GetClaims()
	if claims == nil {
		return nil, errors.New("JWT claims not found")
	}
	
	// Extract role using the same claim path as configured in rbac.json
	// Example: if jwtClaimPath is "role"
	role, ok := claims["role"].(string)
	if !ok {
		return nil, errors.New("role not found in JWT claims")
	}
	
	// Use role for business logic (e.g., personalize UI, filter data)
	return map[string]string{"userRole": role}, nil
}
```

**Note**: All authorization is handled automatically by the middleware. Accessing the role in handlers is only for business logic purposes (e.g., personalizing UI, filtering data).

## Permission Naming Conventions

### Recommended Format

Use the format: `resource:action`

- **Resource**: The entity being accessed (e.g., `users`, `posts`, `orders`)
- **Action**: The operation being performed (e.g., `read`, `write`, `delete`, `update`)

### Examples

```
"users:read"      // Read users
"users:write"     // Create/update users
"users:delete"    // Delete users
"posts:read"      // Read posts
"posts:write"     // Create/update posts
"orders:approve"  // Approve orders
"reports:export"  // Export reports
```

**Avoid inconsistent formats**:
- ❌ `"read_users"`, `"writeUsers"`, `"DELETE_POSTS"`
- ✅ `"users:read"`, `"users:write"`, `"posts:delete"`

### Wildcards Not Supported

**Important**: Wildcards are **NOT supported** in permissions. Only exact matches are allowed.

- ❌ `"*:*"` - Does not match all permissions
- ❌ `"users:*"` - Does not match all user permissions
- ✅ `"users:read"` - Exact match only
- ✅ `"users:write"` - Exact match only

If you need multiple permissions, specify them explicitly:
```json
{
  "name": "admin",
  "permissions": ["users:read", "users:write", "users:delete", "posts:read", "posts:write"]
}
```

Or use role inheritance to avoid duplication:
```json
{
  "name": "editor",
  "permissions": ["users:write", "posts:write"],
  "inheritsFrom": ["viewer"]  // Inherits viewer's permissions
}
```

## Common Patterns

### CRUD Permissions

```json
{
  "roles": [
    {
      "name": "admin",
      "permissions": ["users:delete"],
      "inheritsFrom": ["editor"]
    },
    {
      "name": "editor",
      "permissions": ["users:create", "users:update"],
      "inheritsFrom": ["viewer"]
    },
    {
      "name": "viewer",
      "permissions": ["users:read"]
    }
  ],
  "endpoints": [
    {
      "path": "/api/users",
      "methods": ["POST"],
      "requiredPermissions": ["users:create"]
    },
    {
      "path": "/api/users",
      "methods": ["GET"],
      "requiredPermissions": ["users:read"]
    },
    {
      "path": "^/api/users/\\d+$",
      "methods": ["PUT", "PATCH"],
      "requiredPermissions": ["users:update"]
    },
    {
      "path": "^/api/users/\\d+$",
      "methods": ["DELETE"],
      "requiredPermissions": ["users:delete"]
    }
  ]
}
```

### Resource-Specific Permissions

```json
{
  "roles": [
    {
      "name": "admin",
      "permissions": ["own:posts:read", "own:posts:write", "all:posts:read", "all:posts:write"]
    },
    {
      "name": "author",
      "permissions": ["own:posts:read", "own:posts:write"]
    },
    {
      "name": "viewer",
      "permissions": ["own:posts:read", "all:posts:read"]
    }
  ],
  "endpoints": [
    {
      "path": "/api/posts/my-posts",
      "methods": ["GET"],
      "requiredPermissions": ["own:posts:read"]
    },
    {
      "path": "/api/posts",
      "methods": ["GET"],
      "requiredPermissions": ["all:posts:read"]
    }
  ]
}
```

## Best Practices

### Security
- **Never use header-based RBAC for public APIs** - Use JWT-based RBAC
- **Always validate JWT tokens** - Use proper JWKS endpoints with HTTPS
- **Use HTTPS in production** - Protect tokens and headers
- **Monitor audit logs** - Track authorization decisions

### Configuration
- **Use role inheritance** - Avoid duplicating permissions (only specify additional ones)
- **Use consistent naming** - Follow `resource:action` format (e.g., `users:read`, `posts:write`)
- **Group related permissions** - Organize by resource type
- **Version control configs** - Track RBAC changes in git

## Troubleshooting

**Role not being extracted**
- Ensure `roleHeader` or `jwtClaimPath` is set in config file
- For header-based: check that the header is present in requests
- For JWT-based: ensure OAuth middleware is enabled before RBAC

**Permission checks failing**
- Verify `roles[].permissions` is properly configured
- Check that `endpoints[].requiredPermissions` matches your routes correctly
- Ensure role has the required permission (check inherited permissions too)
- Verify route pattern/regex matches exactly
- Check role inheritance - ensure inherited permissions are included

**Permission always denied**
- Check role assignment - verify user's role has the required permission
- Review role permissions - ensure `roles[].permissions` includes the required permission
- Enable debug logging - check audit logs for authorization decisions

**Permission always allowed**
- Check if endpoint is in RBAC config - routes not in config are allowed to proceed
- Check public endpoints - verify endpoint is not marked as `public: true`
- Review endpoint configuration - ensure `endpoints[].requiredPermissions` is set correctly
- Verify permission check - check audit logs to see if permission check is being performed

**JWT role extraction failing**
- Ensure OAuth middleware is enabled before RBAC
- Verify JWT claim path is correct

**Config file not found**
- Ensure config file exists at the specified path
- Or use default paths (`configs/rbac.json`, `configs/rbac.yaml`, `configs/rbac.yml`)

**Route not being protected by RBAC**
- Verify the route is explicitly configured in `endpoints[]` array
- Check that the path pattern matches exactly (case-sensitive)
- Ensure HTTP method matches (or use `["*"]` for all methods)
- Remember: Routes not in RBAC config are allowed to proceed (not blocked)

## How It Works

1. **Role Extraction**: Extracts user role from header (`X-User-Role`) or JWT claims
2. **Endpoint Matching**: Matches request method + path to endpoint configuration
3. **Permission Check**: Verifies role has required permission for the endpoint
4. **Authorization**: Allows or denies request based on permission check

The middleware automatically handles all authorization - you just define routes normally.

### Unmatched Routes Behavior

**Important**: RBAC only enforces authorization for endpoints that are **explicitly configured** in the RBAC config file.

- ✅ **Routes in RBAC config**: Authorization is enforced (requires valid role and permissions)
- ✅ **Routes NOT in RBAC config**: Requests are allowed to proceed to normal route matching
  - If the route exists in your application, it will be handled normally
  - If the route doesn't exist, it will return 404 (route not registered)

**Example**:
```json
{
  "endpoints": [
    {
      "path": "/api/users",
      "methods": ["GET"],
      "requiredPermissions": ["users:read"]
    }
  ]
}
```

In this configuration:
- `GET /api/users` → **RBAC enforced** (requires `users:read` permission)
- `POST /api/users` → **Not in RBAC config** → Allowed to proceed (may return 404 if route doesn't exist)
- `GET /api/posts` → **Not in RBAC config** → Allowed to proceed (may return 404 if route doesn't exist)
- `GET /health` → **Not in RBAC config** → Allowed to proceed (will work if route exists)

This design allows you to:
- Gradually add RBAC protection to specific endpoints
- Keep some routes unprotected (not in RBAC config)
- Let the router handle 404s for non-existent routes

## Related Documentation

- [HTTP Authentication](https://gofr.dev/docs/advanced-guide/http-authentication) - Basic Auth, API Keys, OAuth 2.0
- [HTTP Communication](https://gofr.dev/docs/advanced-guide/http-communication) - Inter-service HTTP calls
- [Middlewares](https://gofr.dev/docs/advanced-guide/middlewares) - Custom middleware implementation
