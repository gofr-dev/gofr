# Permission-Based Access Control in GoFr

Permission-Based Access Control (PBAC) extends Role-Based Access Control (RBAC) by providing fine-grained authorization at the permission level. Instead of just checking if a user has a specific role, you can check if they have a specific permission to perform an action.

## Overview

GoFr's permission-based RBAC provides:

- ✅ **Fine-Grained Control** - Define permissions like `users:read`, `users:write`, `users:delete`
- ✅ **Route-to-Permission Mapping** - Map HTTP methods and routes to specific permissions
- ✅ **Role-to-Permission Mapping** - Assign permissions to roles
- ✅ **Fallback to Role-Based** - Automatically falls back to role-based checks when permission mapping is missing
- ✅ **Handler-Level Checks** - Use `gofr.RequirePermission()` for explicit permission checks in handlers

## Quick Start

### Basic Permission Configuration

```go
package main

import (
	"fmt"
	"net/http"
	"gofr.dev/pkg/gofr"
	"gofr.dev/pkg/gofr/rbac"
)

func main() {
	app := gofr.New()

	// Load base RBAC configuration
	config, err := rbac.LoadPermissions("configs/rbac.json")
	if err != nil {
		app.Logger().Error("Failed to load RBAC config: ", err)
		return
	}

	// Configure permission-based access control
	config.PermissionConfig = &rbac.PermissionConfig{
		Permissions: map[string][]string{
			"users:read":   {"admin", "editor", "viewer"},
			"users:write":  {"admin", "editor"},
			"users:delete": {"admin"},
			"posts:read":   {"admin", "author", "viewer"},
			"posts:write":  {"admin", "author"},
		},
		RoutePermissionMap: map[string]string{
			"GET /api/users":    "users:read",
			"POST /api/users":   "users:write",
			"DELETE /api/users": "users:delete",
			"GET /api/posts":    "posts:read",
			"POST /api/posts":   "posts:write",
		},
	}

	// Enable RBAC with permissions
	app.EnableRBACWithPermissions(config, func(req *http.Request, args ...any) (string, error) {
		role := req.Header.Get("X-User-Role")
		if role == "" {
			return "", fmt.Errorf("role header not found")
		}
		return role, nil
	})

	// Example routes
	app.GET("/api/users", getAllUsers)
	app.POST("/api/users", createUser)
	app.DELETE("/api/users", gofr.RequirePermission("users:delete", config.PermissionConfig, deleteUser))
	app.GET("/api/posts", getAllPosts)
	app.POST("/api/posts", createPost)

	app.Run()
}

func getAllUsers(ctx *gofr.Context) (interface{}, error) {
	return map[string]string{"message": "Users list"}, nil
}

func createUser(ctx *gofr.Context) (interface{}, error) {
	return map[string]string{"message": "User created"}, nil
}

func deleteUser(ctx *gofr.Context) (interface{}, error) {
	return map[string]string{"message": "User deleted"}, nil
}

func getAllPosts(ctx *gofr.Context) (interface{}, error) {
	return map[string]string{"message": "Posts list"}, nil
}

func createPost(ctx *gofr.Context) (interface{}, error) {
	return map[string]string{"message": "Post created"}, nil
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

### Permission Flow

1. **Request arrives** → Middleware extracts user role
2. **Permission check** → Checks if route has a permission mapping
3. **Role validation** → Verifies if user's role has the required permission
4. **Fallback** → If no permission mapping exists, falls back to role-based check
5. **Authorization decision** → Allow or deny based on the checks

## Configuration

### Permission Structure

```go
type PermissionConfig struct {
    // Permissions maps permission names to roles that have them
    Permissions map[string][]string
    
    // RoutePermissionMap maps "METHOD /path" to permission names
    RoutePermissionMap map[string]string
}
```

### Example Configuration

```go
config.PermissionConfig = &rbac.PermissionConfig{
    Permissions: map[string][]string{
        // Permission: [roles that have this permission]
        "users:read":   {"admin", "editor", "viewer"},
        "users:write":  {"admin", "editor"},
        "users:delete": {"admin"},
        "posts:read":   {"admin", "author", "viewer"},
        "posts:write":  {"admin", "author"},
        "posts:delete": {"admin"},
    },
    RoutePermissionMap: map[string]string{
        // "METHOD /path": "permission"
        "GET /api/users":    "users:read",
        "POST /api/users":   "users:write",
        "DELETE /api/users": "users:delete",
        "GET /api/posts":    "posts:read",
        "POST /api/posts":   "posts:write",
        "DELETE /api/posts": "posts:delete",
    },
}
```

### JSON Configuration (Alternative)

You can also define permissions in the JSON config file:

```json
{
  "permissions": {
    "permissions": {
      "users:read": ["admin", "editor", "viewer"],
      "users:write": ["admin", "editor"],
      "users:delete": ["admin"]
    },
    "routePermissions": {
      "GET /api/users": "users:read",
      "POST /api/users": "users:write",
      "DELETE /api/users": "users:delete"
    }
  },
  "enablePermissions": true
}
```

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

### Alternative Formats

You can use any format that works for your application:

```go
"read_users"      // Alternative format
"write_users"
"delete_users"
"user.read"       // Dot notation
"user.write"
"user.delete"
```

## Route-to-Permission Mapping

### Format

The route permission map uses the format: `"METHOD /path"`

```go
RoutePermissionMap: map[string]string{
    "GET /api/users":      "users:read",
    "POST /api/users":     "users:write",
    "PUT /api/users/:id":  "users:write",
    "DELETE /api/users/:id": "users:delete",
}
```

### Wildcard Support

Route patterns support wildcards:

```go
RoutePermissionMap: map[string]string{
    "GET /api/users/*":    "users:read",   // All GET requests under /api/users/
    "POST /api/*":         "admin:write",  // All POST requests under /api/
    "* /api/admin/*":      "admin:all",    // All methods under /api/admin/
}
```

### Method Matching

- `GET` - Matches GET requests
- `POST` - Matches POST requests
- `PUT` - Matches PUT requests
- `DELETE` - Matches DELETE requests
- `PATCH` - Matches PATCH requests
- `*` - Matches any HTTP method

## Handler-Level Permission Checks

### Using `gofr.RequirePermission()`

For explicit permission checks in handlers:

```go
app.DELETE("/api/users/:id", gofr.RequirePermission("users:delete", config.PermissionConfig, deleteUser))
```

### Example

```go
func deleteUser(ctx *gofr.Context) (interface{}, error) {
    // This handler will only be called if the user has "users:delete" permission
    userID := ctx.PathParam("id")
    
    // Delete user logic here
    return map[string]string{"message": "User deleted"}, nil
}
```

### Multiple Permission Checks

You can chain permission checks:

```go
app.PUT("/api/users/:id", 
    gofr.RequirePermission("users:write", config.PermissionConfig,
        gofr.RequirePermission("users:update", config.PermissionConfig, updateUser)))
```

## Why Load from File When Defining Permissions in Code?

This is a common question when working with permission-based RBAC in GoFr. Let's understand the design rationale.

### The Config File Serves Multiple Purposes

#### 1. **Fallback Authorization**

The `RouteWithPermissions` in the config file acts as a **fallback** when:
- Permission mapping is not found for a route
- You want to use both permission-based AND role-based checks together
- Routes don't have explicit permission mappings

**Example**:
```json
{
  "route": {
    "/api/*": ["admin", "editor"]  // Fallback: any route under /api/* requires admin or editor
  },
  "enablePermissions": true
}
```

Even if a route like `/api/unknown` doesn't have a permission mapping, it will still be checked against the role-based fallback (`/api/*` requires admin/editor).

#### 2. **Other Configuration Settings**

The config file contains important settings beyond permissions:

```json
{
  "route": {
    "/api/*": ["admin", "editor"]
  },
  "overrides": {
    "/health": true,        // Public routes
    "/metrics": true
  },
  "defaultRole": "viewer",  // Default when role extraction fails
  "roleHierarchy": {        // Role inheritance
    "admin": ["editor", "viewer"]
  },
  "enablePermissions": true,  // Enable permission checks
  "enableCache": true,        // Enable caching
  "cacheTTL": "5m"            // Cache TTL
}
```

#### 3. **Configuration Management Benefits**

- **Hot-Reload**: Configuration can be reloaded without code changes
- **Environment Overrides**: Supports `RBAC_ROUTE_*` environment variables
- **Separation of Concerns**: Route-level config in file, fine-grained permissions in code
- **Version Control**: Config changes tracked separately from code

### Alternative: Pure Code-Based Configuration

You can also create the config entirely in code without loading from a file:

```go
config := &rbac.Config{
    RouteWithPermissions: map[string][]string{
        "/api/*": {"admin", "editor"},
    },
    OverRides: map[string]bool{
        "/health": true,
    },
    DefaultRole: "viewer",
    EnablePermissions: true,
    PermissionConfig: &rbac.PermissionConfig{
        Permissions: map[string][]string{
            "users:read": {"admin", "editor", "viewer"},
            "users:write": {"admin", "editor"},
        },
        RoutePermissionMap: map[string]string{
            "GET /api/users": "users:read",
            "POST /api/users": "users:write",
        },
    },
}

app.EnableRBACWithConfig(config)
```

### When to Use Each Approach

#### Use File-Based Config When:
- ✅ You want hot-reload capability
- ✅ Configuration changes frequently
- ✅ Non-developers need to modify permissions
- ✅ You want environment variable overrides
- ✅ You need fallback role-based authorization

#### Use Pure Code-Based Config When:
- ✅ Permissions are static and rarely change
- ✅ All configuration is managed in code
- ✅ You don't need hot-reload
- ✅ You want everything in one place

### Recommended Pattern

For permission-based RBAC, the recommended pattern is:

1. **Minimal config file** with route-level fallbacks and settings:
   ```json
   {
     "route": {
       "/api/*": ["admin", "editor"]
     },
     "enablePermissions": true,
     "overrides": {
       "/health": true
     }
   }
   ```

2. **Fine-grained permissions in code**:
   ```go
   config.PermissionConfig = &rbac.PermissionConfig{
       Permissions: map[string][]string{
           "users:read": {"admin", "editor", "viewer"},
           "users:write": {"admin", "editor"},
       },
       RoutePermissionMap: map[string]string{
           "GET /api/users": "users:read",
           "POST /api/users": "users:write",
       },
   }
   ```

This gives you:
- **Flexibility**: Fine-grained permissions in code (easy to maintain)
- **Safety**: Route-level fallback from config file
- **Hot-Reload**: Can reload route-level config without code changes
- **Best of Both**: Code for complex logic, file for simple route rules

## Implementation Patterns

### 1. Permission-Based RBAC (Header)

**Best for**: Fine-grained access control with header-based roles

**Example**: [Permission-Based RBAC (Header) Example](https://github.com/gofr-dev/gofr/tree/main/examples/rbac/permissions-header)

```go
app.EnableRBACWithPermissions(config, func(req *http.Request, args ...any) (string, error) {
    role := req.Header.Get("X-User-Role")
    if role == "" {
        return "", fmt.Errorf("role header not found")
    }
    return role, nil
})
```

### 2. Permission-Based RBAC (JWT)

**Best for**: Public APIs requiring fine-grained permissions

**Example**: [Permission-Based RBAC (JWT) Example](https://github.com/gofr-dev/gofr/tree/main/examples/rbac/permissions-jwt)

```go
app.EnableOAuth("https://auth.example.com/.well-known/jwks.json", 10)

jwtExtractor := rbac.NewJWTRoleExtractor("role")
config.RoleExtractorFunc = jwtExtractor.ExtractRole

app.EnableRBACWithPermissions(config, jwtExtractor.ExtractRole)
```

### 3. Permission-Based RBAC (Database)

**Best for**: Dynamic roles, multi-tenant applications, admin-managed roles

**Example**: [Permission-Based RBAC (Database) Example](https://github.com/gofr-dev/gofr/tree/main/examples/rbac/permissions-db)

```go
config.RoleExtractorFunc = func(req *http.Request, args ...any) (string, error) {
    userID := req.Header.Get("X-User-ID")
    if userID == "" {
        return "", fmt.Errorf("user ID not found")
    }

    // Get container from args (automatically injected for database-based extraction)
    if len(args) > 0 {
        if cntr, ok := args[0].(*container.Container); ok && cntr != nil && cntr.SQL != nil {
            var role string
            err := cntr.SQL.QueryRowContext(req.Context(), "SELECT role FROM users WHERE id = ?", userID).Scan(&role)
            if err != nil {
                return "", err
            }
            return role, nil
        }
    }
    
    return "", fmt.Errorf("database not available")
}

app.EnableRBACWithPermissions(config, config.RoleExtractorFunc)
```

> **Note**: The container is automatically passed to `RoleExtractorFunc` when using database-based role extraction. For header-based or JWT-based RBAC, the container is not needed and `args` will be empty.

## Best Practices

### 1. Use Consistent Permission Naming

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

### 2. Group Related Permissions

```go
Permissions: map[string][]string{
    // User management
    "users:read":   {"admin", "editor", "viewer"},
    "users:write":  {"admin", "editor"},
    "users:delete": {"admin"},
    
    // Post management
    "posts:read":   {"admin", "author", "viewer"},
    "posts:write":  {"admin", "author"},
    "posts:delete": {"admin"},
    
    // Order management
    "orders:read":    {"admin", "manager", "viewer"},
    "orders:approve": {"admin", "manager"},
    "orders:cancel":  {"admin"},
}
```

### 3. Use Fallback Routes

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

### 4. Document Permission Requirements

Document which permissions are required for each endpoint:

```go
// GET /api/users - Requires: users:read
// POST /api/users - Requires: users:write
// DELETE /api/users/:id - Requires: users:delete
```

### 5. Test Permission Checks

Write integration tests to verify permission checks:

```go
func TestPermissionChecks(t *testing.T) {
    // Test that admin can delete users
    // Test that editor cannot delete users
    // Test that viewer cannot write users
}
```

## Common Patterns

### Pattern 1: CRUD Permissions

```go
Permissions: map[string][]string{
    "users:create": {"admin", "editor"},
    "users:read":   {"admin", "editor", "viewer"},
    "users:update": {"admin", "editor"},
    "users:delete": {"admin"},
},
RoutePermissionMap: map[string]string{
    "POST /api/users":     "users:create",
    "GET /api/users":      "users:read",
    "PUT /api/users/:id":  "users:update",
    "DELETE /api/users/:id": "users:delete",
},
```

### Pattern 2: Hierarchical Permissions

```go
Permissions: map[string][]string{
    "users:read":   {"admin", "editor", "viewer"},
    "users:write":  {"admin", "editor"},      // Includes read implicitly
    "users:delete": {"admin"},                // Includes read and write implicitly
},
```

### Pattern 3: Resource-Specific Permissions

```go
Permissions: map[string][]string{
    "own:posts:read":   {"admin", "author", "viewer"},
    "own:posts:write":  {"admin", "author"},
    "all:posts:read":  {"admin", "editor"},
    "all:posts:write":  {"admin", "editor"},
},
```

## Troubleshooting

### Permission Check Not Working

1. **Verify `enablePermissions` is true** in config
2. **Check route format** - Must be `"METHOD /path"` (e.g., `"GET /api/users"`)
3. **Verify role extraction** - Ensure role is being extracted correctly
4. **Check permission mapping** - Ensure route has a permission mapping
5. **Review fallback** - Check if role-based fallback is allowing access

### Permission Always Denied

1. **Check role assignment** - Verify user's role has the required permission
2. **Review permission config** - Ensure `PermissionConfig` is properly set
3. **Check route matching** - Verify route pattern matches exactly
4. **Enable debug logging** - Check audit logs for authorization decisions

### Permission Always Allowed

1. **Check fallback routes** - Fallback might be too permissive
2. **Verify permission check** - Ensure `EnablePermissions` is true
3. **Review route mapping** - Ensure route has a permission mapping

## Related Documentation

- [Role-Based Access Control (RBAC)](https://gofr.dev/docs/advanced-guide/rbac) - Complete RBAC guide
- [HTTP Authentication](https://gofr.dev/docs/advanced-guide/http-authentication) - Authentication methods
- [Permission-Based RBAC Examples](https://github.com/gofr-dev/gofr/tree/main/examples/rbac) - Working examples

## Examples

- [Permission-Based RBAC (Header)](https://github.com/gofr-dev/gofr/tree/main/examples/rbac/permissions-header) - Header-based role extraction with permissions
- [Permission-Based RBAC (JWT)](https://github.com/gofr-dev/gofr/tree/main/examples/rbac/permissions-jwt) - JWT-based role extraction with permissions
- [Permission-Based RBAC (Database)](https://github.com/gofr-dev/gofr/tree/main/examples/rbac/permissions-db) - Database-based role extraction with permissions

Each example includes:
- Complete working code
- Configuration files
- Integration tests
- Detailed README with setup instructions

