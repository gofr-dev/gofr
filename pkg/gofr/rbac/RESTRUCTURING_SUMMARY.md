# RBAC Restructuring Summary

## Overview

The RBAC module has been restructured to follow the same pattern as `DBResolver` and other datasource providers in GoFr. This ensures consistency across the framework and provides a unified way to declare RBAC configurations.

## Key Changes

### 1. Provider Pattern Alignment

**Before:**
```go
provider := rbac.NewProvider()
app.EnableRBAC(provider, "configs/rbac.json", options...)
```

**After:**
```go
provider := rbac.NewProvider(app, "configs/rbac.json", options...)
app.AddRBAC(provider)
```

### 2. Dependency Injection

The RBAC provider now implements the standard `provider` interface:

```go
type provider interface {
    UseLogger(logger any)
    UseMetrics(metrics any)
    UseTracer(tracer any)
    Connect()
}
```

This matches the pattern used by:
- `DBResolverProvider`
- `MongoProvider`
- `PostgresProvider`
- Other datasource providers

### 3. Unified Configuration Format

A new unified configuration format has been introduced that follows industry best practices:

#### Roles with Attributes
```json
{
  "roles": [
    {
      "name": "editor",
      "permissions": ["users:read", "users:write"],
      "attributes": {
        "department": ["engineering", "content"],
        "region": ["us-east"]
      },
      "inheritsFrom": ["viewer"]
    }
  ]
}
```

#### Endpoint Mapping
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
    }
  ]
}
```

## Differences from DBResolver

While RBAC now follows the same pattern as DBResolver, there are some differences:

| Aspect | DBResolver | RBAC |
|--------|-----------|------|
| **Resource Replacement** | Replaces `container.SQL` | Adds middleware, stores provider |
| **Config Source** | Config struct passed to constructor | Config file path + options |
| **Initialization** | Creates resolver from primary DB | Loads config from file |
| **Middleware** | Optional (for context injection) | Required (for authorization) |

## Benefits

1. **Consistency**: Same pattern across all GoFr modules
2. **Dependency Injection**: Proper logger, metrics, and tracer injection
3. **Unified Configuration**: Single way to declare RBAC with roles and endpoints
4. **Industry Best Practices**: Follows AWS IAM, Google Cloud IAM patterns
5. **Attribute-Based Access Control**: Support for role attributes (ABAC)
6. **Backward Compatibility**: Legacy formats still supported

## Migration Guide

### Old Way (Still Supported)
```go
provider := rbac.NewProvider()
app.EnableRBAC(provider, "configs/rbac.json", &rbac.JWTExtractor{Claim: "role"})
```

### New Way (Recommended)
```go
// Option 1: Direct provider creation
provider := rbac.NewProvider(app, "configs/rbac.json", &rbac.JWTExtractor{Claim: "role"})
app.AddRBAC(provider)

// Option 2: Convenience function
err := rbac.InitRBAC(app, "configs/rbac.json", &rbac.JWTExtractor{Claim: "role"})
```

## Configuration Format

### Unified Format (Recommended)

```json
{
  "roleHeader": "X-User-Role",
  "roles": [
    {
      "name": "admin",
      "permissions": ["*:*"]
    },
    {
      "name": "editor",
      "permissions": ["users:read", "users:write"],
      "inheritsFrom": ["viewer"]
    }
  ],
  "endpoints": [
    {
      "path": "/api/users",
      "methods": ["GET"],
      "requiredPermission": "users:read"
    }
  ]
}
```

### Legacy Format (Still Supported)

```json
{
  "roleHeader": "X-User-Role",
  "route": {
    "/api/users": ["admin", "editor"]
  }
}
```

## Endpoint Mapping Strategy

Based on industry best practices:

1. **Permission-Based** (Recommended): Map endpoints to permissions, assign permissions to roles
2. **Role-Based**: Map endpoints directly to roles
3. **Combined**: Use both for maximum flexibility

See `UNIFIED_RBAC_GUIDE.md` for detailed examples and best practices.

