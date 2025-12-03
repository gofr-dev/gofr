# Unified RBAC Configuration Guide

This guide explains the **unified RBAC configuration format** - the **ONLY way** to declare RBAC in GoFr. This follows industry best practices from AWS IAM, Google Cloud IAM, and other major platforms.

## Overview

**GoFr RBAC is configuration-only** - there are no programmatic functions, no REST API endpoints for management, and no code-based authorization rules. All RBAC is declared through configuration files.

The unified RBAC configuration provides a single, consistent way to declare:
1. **Roles** with their permissions and attributes
2. **Endpoints** with their access requirements (REST API mapping)
3. **Role-to-attributes mapping** for attribute-based access control (ABAC)

## Why Configuration-Only?

- ✅ **Single Source of Truth**: All authorization rules in config files
- ✅ **Version Control**: Track all RBAC changes in git
- ✅ **No Code Changes**: Update authorization without redeploying
- ✅ **Infrastructure as Code**: Declarative RBAC configuration
- ✅ **Audit Compliance**: Complete history of authorization changes
- ✅ **Environment Parity**: Same config across all environments

## Key Features

### 1. Role-Based with Attributes (RBAC + ABAC)

Roles can have attributes that enable fine-grained access control:

```json
{
  "roles": [
    {
      "name": "editor",
      "permissions": ["users:read", "users:write"],
      "attributes": {
        "department": ["engineering", "content"],
        "region": ["us-east", "eu-west"],
        "environment": ["production", "staging"]
      }
    }
  ]
}
```

### 2. Unified Endpoint Mapping

Endpoints are mapped using a clear, declarative format:

```json
{
  "endpoints": [
    {
      "path": "/api/users",
      "methods": ["GET"],
      "requiredPermission": "users:read"
    },
    {
      "path": "/api/users",
      "methods": ["POST", "PUT"],
      "requiredPermission": "users:write"
    },
    {
      "path": "/health",
      "methods": ["GET"],
      "public": true
    }
  ]
}
```

### 3. Role Inheritance

Roles can inherit permissions from other roles. **You only need to specify the additional permissions** - inherited permissions are automatically included:

```json
{
  "roles": [
    {
      "name": "editor",
      "permissions": ["users:write", "posts:write"],  // Only additional permissions
      "inheritsFrom": ["viewer"]  // Automatically gets viewer's permissions
    },
    {
      "name": "viewer",
      "permissions": ["users:read", "posts:read"]
    }
  ]
}
```

**Result**: Editor automatically has `["users:read", "posts:read", "users:write", "posts:write"]` - no need to duplicate viewer's permissions!

## Configuration Structure

### Roles

```json
{
  "roles": [
    {
      "name": "string",              // Required: Role name
      "permissions": ["string"],      // Optional: Permissions (format: "resource:action")
      "attributes": {                 // Optional: Role attributes for ABAC
        "department": ["string"],
        "region": ["string"],
        "environment": ["string"],
        "custom": {
          "key": ["value"]
        }
      },
      "inheritsFrom": ["string"]      // Optional: Roles to inherit from
    }
  ]
}
```

### Endpoints

```json
{
  "endpoints": [
    {
      "methods": ["string"],          // Required: HTTP methods (GET, POST, PUT, DELETE, etc.)
      "path": "string",               // Optional: Route path (supports wildcards: /api/*)
      "regex": "string",               // Optional: Regex pattern (takes precedence over path)
      "requiredPermission": "string",  // Optional: Required permission (format: "resource:action")
      "allowedRoles": ["string"],     // Optional: Allowed roles
      "public": false                 // Optional: Public endpoint (bypasses auth)
    }
  ]
}
```

## Endpoint Mapping Strategy

Based on industry best practices (AWS API Gateway, Google Cloud Endpoints, etc.):

### 1. Permission-Based (Recommended)

Map endpoints to permissions, then assign permissions to roles:

```json
{
  "roles": [
    {
      "name": "editor",
      "permissions": ["users:read", "users:write"]
    }
  ],
  "endpoints": [
    {
      "path": "/api/users",
      "methods": ["GET"],
      "requiredPermission": "users:read"
    },
    {
      "path": "/api/users",
      "methods": ["POST"],
      "requiredPermission": "users:write"
    }
  ]
}
```

**Benefits:**
- Fine-grained control
- Easy to audit
- Follows principle of least privilege
- Matches AWS IAM, Google Cloud IAM patterns
- Scales well as API grows

### 2. Role-Based

Map endpoints directly to roles:

```json
{
  "endpoints": [
    {
      "path": "/api/admin/*",
      "methods": ["*"],
      "allowedRoles": ["admin"]
    }
  ]
}
```

**Benefits:**
- Simple setup
- Quick to implement
- Good for small APIs

**Note**: If you're using permission-based access control (roles have permissions), prefer `requiredPermission` instead. For example, if admin has `*:*` permissions, you can use:
```json
{
  "endpoints": [
    {
      "path": "/api/admin/*",
      "methods": ["*"],
      "requiredPermission": "admin:*"  // *:* matches this
    }
  ]
}
```

This way, the `*:*` permission automatically grants access without needing to specify `allowedRoles` separately.

### 3. Combined (Permission + Role)

Use both for maximum flexibility:

```json
{
  "endpoints": [
    {
      "path": "/api/users",
      "methods": ["DELETE"],
      "requiredPermission": "users:delete",
      "allowedRoles": ["admin"]
    }
  ]
}
```

**Note:** When both are specified, both checks must pass (AND logic).

## Best Practices

### 1. Use RESTful Permission Naming

Follow the pattern: `resource:action`

- `users:read` - Read users
- `users:write` - Create/update users
- `users:delete` - Delete users
- `posts:read` - Read posts
- `posts:write` - Create/update posts

### 2. Use Wildcards for Admin Roles

```json
{
  "name": "admin",
  "permissions": ["*:*"]
}
```

### 3. Define Public Endpoints Explicitly

```json
{
  "endpoints": [
    {
      "path": "/health",
      "methods": ["GET"],
      "public": true
    }
  ]
}
```

### 4. Use Regex for Complex Patterns

```json
{
  "endpoints": [
    {
      "regex": "^/api/users/\\d+$",
      "methods": ["GET"],
      "requiredPermission": "users:read"
    }
  ]
}
```

### 5. Leverage Role Inheritance

```json
{
  "roles": [
    {
      "name": "admin",
      "permissions": ["*:*"]
    },
    {
      "name": "editor",
      "permissions": ["users:write"],
      "inheritsFrom": ["viewer"]
    },
    {
      "name": "viewer",
      "permissions": ["users:read"]
    }
  ]
}
```

## Migration from Legacy Format

The new format is backward compatible. Legacy formats are still supported:

### Legacy Route Mapping
```json
{
  "route": {
    "/api/users": ["admin", "editor"]
  }
}
```

### New Endpoint Mapping
```json
{
  "endpoints": [
    {
      "path": "/api/users",
      "methods": ["*"],
      "allowedRoles": ["admin", "editor"]
    }
  ]
}
```

## Example: Complete Configuration

See `example-rbac-unified.json` for a complete example configuration.

## Usage

```go
import (
    "gofr.dev/pkg/gofr"
    "gofr.dev/pkg/gofr/rbac"
)

func main() {
    app := gofr.New()
    
    // Create RBAC provider (follows same pattern as DBResolver)
    provider := rbac.NewProvider(app, "configs/rbac.json", &rbac.JWTExtractor{Claim: "role"})
    
    // Add RBAC - all authorization is handled through config file
    app.AddRBAC(provider)
    
    // Define your REST API routes - authorization is automatic based on config
    app.GET("/api/users", getAllUsers)        // Auto-checked: users:read
    app.POST("/api/users", createUser)        // Auto-checked: users:write
    app.DELETE("/api/users/:id", deleteUser) // Auto-checked: users:delete
    
    app.Run()
}
```

**Important**: All authorization rules are defined in `configs/rbac.json` using the unified `Roles` + `Endpoints` format. There are no programmatic functions, no deprecated fields (`route`, `overrides`, `roleHierarchy`, `permissions`), and no code-based authorization rules - everything is config-based.

## Comparison with DBResolver Pattern

The RBAC implementation now follows the same pattern as DBResolver:

| Aspect | DBResolver | RBAC (New) |
|--------|-----------|------------|
| Provider Creation | `NewDBResolverProvider(app, cfg)` | `NewProvider(app, configFile, options...)` |
| Dependency Injection | `UseLogger()`, `UseMetrics()`, `UseTracer()` | `UseLogger()`, `UseMetrics()`, `UseTracer()` |
| Initialization | `Connect()` | `Connect()` |
| App Integration | `app.AddDBResolver(provider)` | `app.AddRBAC(provider)` |
| Convenience Function | `InitDBResolver(app, cfg)` | `InitRBAC(app, configFile, options...)` |

This ensures consistency across GoFr's datasource and security modules.

