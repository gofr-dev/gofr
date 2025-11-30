# Permission-Based Access Control in GoFr

Permission-Based Access Control (PBAC) extends Role-Based Access Control (RBAC) by providing fine-grained authorization at the permission level. Instead of just checking if a user has a specific role, you can check if they have a specific permission to perform an action.

## Overview

GoFr's permission-based RBAC provides:

- ✅ **Factory Function Pattern** - `app.EnableRBAC()` follows the same pattern as `app.AddMongo()`, `app.AddPostgres()` - automatically registers RBAC when called
- ✅ **Fine-Grained Control** - Define permissions like `users:read`, `users:write`, `users:delete`
- ✅ **Structured Route Rules** - Map HTTP methods and routes to specific permissions using flexible regex patterns
- ✅ **Role-Centric Permissions** - Intuitive model where roles define what they can do
- ✅ **Automatic Route Protection** - Middleware automatically checks permissions - no route-level wrappers needed
- ✅ **Fallback to Role-Based** - Automatically falls back to role-based checks when permission mapping is missing

> **Note**: `app.EnableRBAC()` follows the same factory function pattern used throughout GoFr for datasource registration. Just like you use `app.AddMongo(db)` to register MongoDB, you use `app.EnableRBAC()` to register and configure RBAC. When using RBAC options (e.g., `&rbac.JWTExtractor{}`), you must import the rbac package: `import "gofr.dev/pkg/gofr/rbac"`.

## Quick Start

### Basic Permission Configuration

```go
package main

import (
	"gofr.dev/pkg/gofr"
)

func main() {
	app := gofr.New()

	// Enable RBAC with permissions
	// Config file defines all permissions and route mappings
	// EnableRBAC is a factory function that registers RBAC automatically
	app.EnableRBAC() // Uses configs/rbac.json by default

	// Example routes - permissions checked automatically by middleware
	app.GET("/api/users", getAllUsers)        // Auto-checked: users:read
	app.POST("/api/users", createUser)       // Auto-checked: users:write
	app.DELETE("/api/users/:id", deleteUser) // Auto-checked: users:delete

	app.Run()
}
```

**Configuration** (`configs/rbac.json`):

```json
{
  "roleHeader": "X-User-Role",
  "route": {
    "/api/*": ["admin", "editor"]
  },
  "permissions": {
    "rolePermissions": {
      "admin": ["users:read", "users:write", "users:delete"],
      "editor": ["users:read", "users:write"],
      "viewer": ["users:read"]
    },
    "routePermissionRules": [
      {
        "methods": ["GET"],
        "regex": "^/api/users(/.*)?$",
        "permission": "users:read"
      },
      {
        "methods": ["POST", "PUT"],
        "regex": "^/api/users(/.*)?$",
        "permission": "users:write"
      },
      {
        "methods": ["DELETE"],
        "regex": "^/api/users/\\d+$",
        "permission": "users:delete"
      }
    ]
  }
}
```

## How Permissions Work

### Two-Tier Authorization System

GoFr's RBAC middleware implements a **two-tier authorization system**:

1. **Permission-Based Check** (Primary) - Checks if user's role has the required permission
2. **Role-Based Check** (Fallback) - Falls back to route-based role checking if permission check fails

```go
// Check permission-based access if enabled
if config.EnablePermissions && config.PermissionConfig != nil {
    if err := CheckPermission(reqWithRole, config.PermissionConfig); err == nil {
        authorized = true
        authReason = "permission-based"
    }
}

// Check role-based access (if not already authorized by permissions)
if !authorized {
    if isRoleAllowed(role, route, config) {
        authorized = true
        authReason = "role-based"
    }
}
```

**Benefits of Fallback**:
- Provides safety net when permission mappings are missing
- Allows gradual migration from role-based to permission-based
- Ensures routes are always protected even if permission config is incomplete

### Permission Flow

1. **Request arrives** → Middleware extracts user role
2. **Permission check** → Checks if route matches a permission rule
3. **Role validation** → Verifies if user's role has the required permission
4. **Fallback** → If no permission mapping exists, falls back to role-based check
5. **Authorization decision** → Allow or deny based on the checks

## Configuration

### Role-Centric Permission Model

GoFr uses a **role-centric** permission model, which is more intuitive than the traditional permission-centric model:

```json
{
  "rolePermissions": {
    "admin": ["users:read", "users:write", "users:delete"],
    "editor": ["users:read", "users:write"],
    "viewer": ["users:read"]
  }
}
```

**Why Role-Centric?**
- ✅ Easy to see what a role can do (all permissions in one place)
- ✅ Better for understanding role capabilities
- ✅ More maintainable as roles change
- ✅ Aligns with how developers think about access control

### Structured Route Permission Rules

The new `routePermissionRules` format provides flexible, structured route-to-permission mapping:

```json
{
  "routePermissionRules": [
    {
      "methods": ["GET"],
      "regex": "^/api/users(/.*)?$",
      "permission": "users:read"
    },
    {
      "methods": ["POST", "PUT"],
      "path": "/api/users",
      "permission": "users:write"
    },
    {
      "methods": ["DELETE"],
      "regex": "^/api/users/\\d+$",
      "permission": "users:delete"
    }
  ]
}
```

**Fields**:
- `methods` (array): HTTP methods (GET, POST, PUT, DELETE, PATCH, etc.). Empty or `["*"]` matches all methods
- `path` (string): Path pattern (supports wildcards). Used when `regex` is not provided
- `regex` (string): Regular expression pattern. Takes precedence over `path` if both are provided
- `permission` (string): Required permission for matching routes

**Benefits**:
- ✅ Supports multiple HTTP methods per rule
- ✅ Flexible regex patterns for complex route matching
- ✅ More readable than string-based `"METHOD /path"` format
- ✅ Easier to maintain and extend

### Legacy Format Support

For backward compatibility, the legacy `routePermissions` format is still supported:

```json
{
  "routePermissions": {
    "GET /api/users": "users:read",
    "POST /api/users": "users:write",
    "DELETE /api/users": "users:delete"
  }
}
```

> **Note**: `routePermissionRules` takes precedence over `routePermissions` if both are provided.

## Permission Naming Conventions

### Recommended Format

Use the format: `resource:action`

- **Resource**: The entity being accessed (e.g., `users`, `posts`, `orders`)
- **Action**: The operation being performed (e.g., `read`, `write`, `delete`, `update`)

### Examples

```go
"users:read"      // Read users
"users:write"     // Create/update users
"users:delete"    // Delete users
"posts:read"      // Read posts
"posts:write"     // Create/update posts
"orders:approve"  // Approve orders
"reports:export"  // Export reports
```

## Automatic Route Protection

With the new API, middleware automatically checks permissions based on `routePermissionRules`. You **don't need** to add `RequirePermission()` at the route level:

```go
// ✅ Good: Middleware automatically checks permissions
app.GET("/api/users", getAllUsers)
app.POST("/api/users", createUser)
app.DELETE("/api/users/:id", deleteUser)

// ❌ Not needed: Middleware already checks permissions
// app.DELETE("/api/users/:id", gofr.RequirePermission("users:delete", config, deleteUser))
```

**When to use `RequirePermission()`**:
- For programmatic checks within handlers
- For dynamic, conditional permissions
- For fine-grained checks that can't be expressed in route rules

## Implementation Patterns

### 1. Permission-Based RBAC (Header)

**Best for**: Fine-grained access control with header-based roles

**Example**: [Permission-Based RBAC (Header) Example](https://github.com/gofr-dev/gofr/tree/main/examples/rbac/permissions-header)

```go
app := gofr.New()

// Enable RBAC - config file defines roleHeader and permissions
app.EnableRBAC() // Uses configs/rbac.json by default
```

### 2. Permission-Based RBAC (JWT)

**Best for**: Public APIs requiring fine-grained permissions

**Example**: [Permission-Based RBAC (JWT) Example](https://github.com/gofr-dev/gofr/tree/main/examples/rbac/permissions-jwt)

```go
app := gofr.New()

// Enable OAuth middleware
app.EnableOAuth("https://auth.example.com/.well-known/jwks.json", 10)

// Enable RBAC with JWT and permissions
app.EnableRBAC("", &rbac.JWTExtractor{Claim: "role"})
```

## Common Patterns

### Pattern 1: CRUD Permissions

```json
{
  "rolePermissions": {
    "admin": ["users:create", "users:read", "users:update", "users:delete"],
    "editor": ["users:create", "users:read", "users:update"],
    "viewer": ["users:read"]
  },
  "routePermissionRules": [
    {
      "methods": ["POST"],
      "regex": "^/api/users$",
      "permission": "users:create"
    },
    {
      "methods": ["GET"],
      "regex": "^/api/users(/.*)?$",
      "permission": "users:read"
    },
    {
      "methods": ["PUT", "PATCH"],
      "regex": "^/api/users/\\d+$",
      "permission": "users:update"
    },
    {
      "methods": ["DELETE"],
      "regex": "^/api/users/\\d+$",
      "permission": "users:delete"
    }
  ]
}
```

### Pattern 2: Resource-Specific Permissions

```json
{
  "rolePermissions": {
    "admin": ["own:posts:read", "own:posts:write", "all:posts:read", "all:posts:write"],
    "author": ["own:posts:read", "own:posts:write"],
    "viewer": ["own:posts:read", "all:posts:read"]
  },
  "routePermissionRules": [
    {
      "methods": ["GET"],
      "regex": "^/api/posts/my-posts$",
      "permission": "own:posts:read"
    },
    {
      "methods": ["GET"],
      "regex": "^/api/posts(/.*)?$",
      "permission": "all:posts:read"
    }
  ]
}
```

## Best Practices

### 1. Use Role-Centric Permissions

```json
{
  "rolePermissions": {
    "admin": ["users:read", "users:write", "users:delete"],
    "editor": ["users:read", "users:write"],
    "viewer": ["users:read"]
  }
}
```

**Benefits**:
- Easy to see what each role can do
- Better for understanding role capabilities
- More maintainable

### 2. Use Structured Route Rules

```json
{
  "routePermissionRules": [
    {
      "methods": ["GET"],
      "regex": "^/api/users(/.*)?$",
      "permission": "users:read"
    }
  ]
}
```

**Benefits**:
- Supports multiple HTTP methods
- Flexible regex patterns
- More readable and maintainable

### 3. Use Consistent Permission Naming

```go
// Good: Consistent format
"users:read"
"users:write"
"users:delete"
"posts:read"
"posts:write"

// Avoid: Inconsistent formats
"read_users"
"writeUsers"
"DELETE_POSTS"
```

### 4. Group Related Permissions

```json
{
  "rolePermissions": {
    "admin": [
      "users:read", "users:write", "users:delete",
      "posts:read", "posts:write", "posts:delete",
      "orders:read", "orders:approve", "orders:cancel"
    ]
  }
}
```

### 5. Use Fallback Routes

Always define fallback routes in your config file:

```json
{
  "route": {
    "/api/*": ["admin", "editor"],
    "*": ["viewer"]
  }
}
```

This ensures routes without explicit permission mappings are still protected.

### 6. Document Permission Requirements

Document which permissions are required for each endpoint:

```go
// GET /api/users - Requires: users:read
// POST /api/users - Requires: users:write
// DELETE /api/users/:id - Requires: users:delete
```

### 7. Test Permission Checks

Write integration tests to verify permission checks:

```go
func TestPermissionChecks(t *testing.T) {
    // Test that admin can delete users
    // Test that editor cannot delete users
    // Test that viewer cannot write users
}
```

### 8. Let Middleware Handle Checks

Don't add `RequirePermission()` at route level unless needed for programmatic checks:

```go
// ✅ Good: Middleware automatically checks
app.DELETE("/api/users/:id", deleteUser)

// ❌ Not needed: Middleware already checks
// app.DELETE("/api/users/:id", gofr.RequirePermission("users:delete", config, deleteUser))
```

## Troubleshooting

### Permission Check Not Working

1. **Verify `rolePermissions` is configured** - Check that roles have permissions assigned
2. **Check route rule format** - Ensure `routePermissionRules` match your routes correctly
3. **Verify role extraction** - Ensure role is being extracted correctly
4. **Check permission mapping** - Ensure route matches a permission rule
5. **Review fallback** - Check if role-based fallback is allowing access

### Permission Always Denied

1. **Check role assignment** - Verify user's role has the required permission
2. **Review permission config** - Ensure `rolePermissions` is properly set
3. **Check route matching** - Verify route pattern/regex matches exactly
4. **Enable debug logging** - Check audit logs for authorization decisions

### Permission Always Allowed

1. **Check fallback routes** - Fallback might be too permissive
2. **Verify permission check** - Ensure `PermissionConfig` is set
3. **Review route mapping** - Ensure route matches a permission rule

## Related Documentation

- [Role-Based Access Control (RBAC)](./rbac/page.md) - Complete RBAC guide
- [HTTP Authentication](https://gofr.dev/docs/advanced-guide/http-authentication) - Authentication methods
- [Permission-Based RBAC Examples](https://github.com/gofr-dev/gofr/tree/main/examples/rbac) - Working examples
- [RBAC Architecture](./rbac/ARCHITECTURE.md) - Code execution flow

## Examples

- [Permission-Based RBAC (Header)](https://github.com/gofr-dev/gofr/tree/main/examples/rbac/permissions-header) - Header-based role extraction with permissions
- [Permission-Based RBAC (JWT)](https://github.com/gofr-dev/gofr/tree/main/examples/rbac/permissions-jwt) - JWT-based role extraction with permissions

Each example includes:
- Complete working code
- Configuration files
- Integration tests
- Detailed README with setup instructions
