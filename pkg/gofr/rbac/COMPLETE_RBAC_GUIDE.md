# Complete RBAC Guide - Single Way to Declare RBAC

## Overview

GoFr RBAC uses **only configuration files** - there are no REST API endpoints for RBAC management, no programmatic functions, and no code-based authorization rules. This ensures a single, consistent way to declare all RBAC.

## Key Principles

### 1. Configuration-Only Approach

**No REST API for RBAC Management:**
- RBAC is infrastructure, not application data
- Configuration files are the source of truth
- Changes are made by updating config files and redeploying
- Follows Infrastructure as Code principles

**No Programmatic Functions:**
- No `RequireRole()`, `RequireAnyRole()`, or `RequirePermission()` functions
- All authorization is handled by middleware based on config
- Prevents inconsistent authorization logic across codebase

**Single Source of Truth:**
- All RBAC rules in configuration files (JSON/YAML)
- Version controlled in git
- Same config across all environments

### 2. Role-Based with Attributes

Roles are assigned to attributes, enabling Attribute-Based Access Control (ABAC):

```json
{
  "roles": [
    {
      "name": "editor",
      "permissions": ["users:read", "users:write"],
      "attributes": {
        "department": ["engineering", "content"],
        "region": ["us-east", "eu-west"],
        "environment": ["production", "staging"],
        "custom": {
          "project": ["project-a", "project-b"],
          "team": ["team-1"]
        }
      }
    }
  ]
}
```

**Attributes Enable:**
- Department-based access control
- Regional restrictions
- Environment-specific permissions
- Project-based access
- Custom business logic constraints

### 3. REST API Endpoint Mapping

Endpoints are mapped following RESTful API design principles:

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
      "methods": ["POST"],
      "requiredPermission": "users:write"
    },
    {
      "path": "/api/users/{id}",
      "methods": ["GET"],
      "regex": "^/api/users/\\d+$",
      "requiredPermission": "users:read"
    },
    {
      "path": "/api/users/{id}",
      "methods": ["PUT", "PATCH"],
      "regex": "^/api/users/\\d+$",
      "requiredPermission": "users:write"
    },
    {
      "path": "/api/users/{id}",
      "methods": ["DELETE"],
      "regex": "^/api/users/\\d+$",
      "requiredPermission": "users:delete",
      "allowedRoles": ["admin"]
    }
  ]
}
```

## Industry Best Practices Analysis

### AWS IAM Pattern

**How AWS Does It:**
- IAM Roles with policies
- Resource-based permissions (`arn:aws:execute-api:region:account:api/stage/method/resource`)
- API Gateway authorizers map to IAM permissions
- Conditions for attribute-based access

**GoFr Equivalent:**
```json
{
  "roles": [
    {"name": "admin", "permissions": ["*:*"]}
  ],
  "endpoints": [
    {"path": "/api/users", "methods": ["GET"], "requiredPermission": "users:read"}
  ]
}
```

### Google Cloud IAM Pattern

**How Google Does It:**
- IAM roles (viewer, editor, owner)
- Service-level permissions (`service.resource.action`)
- OpenAPI spec with security requirements
- Conditions for attribute-based access

**GoFr Equivalent:**
```json
{
  "roles": [
    {"name": "editor", "permissions": ["users.read", "users.write"]}
  ],
  "endpoints": [
    {"path": "/api/users", "methods": ["GET"], "requiredPermission": "users.read"}
  ]
}
```

### Microsoft Azure API Management Pattern

**How Azure Does It:**
- Azure AD roles
- XML-based authorization policies
- Policy expressions for role mapping
- Conditions for attribute-based access

**GoFr Equivalent:**
```json
{
  "endpoints": [
    {"path": "/api/admin/*", "methods": ["*"], "allowedRoles": ["admin"]}
  ]
}
```

## Endpoint Mapping Strategy

### Recommended: Permission-Based Mapping

**Best for:** Production applications, large APIs, compliance requirements

```json
{
  "roles": [
    {
      "name": "editor",
      "permissions": ["users:read", "users:write", "posts:read", "posts:write"]
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
- Principle of least privilege
- Matches AWS IAM, Google Cloud IAM patterns
- Scales well as API grows

### Alternative: Direct Role Mapping

**Best for:** Simple APIs, small teams, quick setup

```json
{
  "endpoints": [
    {
      "path": "/api/admin/*",
      "methods": ["*"],
      "allowedRoles": ["admin"]
    },
    {
      "path": "/api/users",
      "methods": ["GET"],
      "allowedRoles": ["admin", "editor", "viewer"]
    }
  ]
}
```

### Combined: Permission + Role

**Best for:** Maximum security, defense in depth, critical operations

```json
{
  "endpoints": [
    {
      "path": "/api/users/{id}",
      "methods": ["DELETE"],
      "regex": "^/api/users/\\d+$",
      "requiredPermission": "users:delete",
      "allowedRoles": ["admin"]
    }
  ]
}
```

**Note:** When both are specified, both checks must pass (AND logic).

## REST API Method Mapping

Follow standard REST conventions:

| HTTP Method | REST Action | Permission | Example Endpoint |
|------------|-------------|------------|------------------|
| GET | Read | `resource:read` | `GET /api/users` → `users:read` |
| POST | Create | `resource:write` | `POST /api/users` → `users:write` |
| PUT | Update (full) | `resource:write` | `PUT /api/users/{id}` → `users:write` |
| PATCH | Update (partial) | `resource:write` | `PATCH /api/users/{id}` → `users:write` |
| DELETE | Delete | `resource:delete` | `DELETE /api/users/{id}` → `users:delete` |

## Complete Example

```json
{
  "roleHeader": "X-User-Role",
  "roles": [
    {
      "name": "admin",
      "permissions": ["*:*"],
      "attributes": {
        "department": ["*"],
        "region": ["*"]
      }
    },
    {
      "name": "editor",
      "permissions": [
        "users:read",
        "users:write",
        "posts:read",
        "posts:write"
      ],
      "attributes": {
        "department": ["engineering", "content"],
        "region": ["us-east", "eu-west"]
      },
      "inheritsFrom": ["viewer"]
    },
    {
      "name": "viewer",
      "permissions": [
        "users:read",
        "posts:read"
      ]
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
      "requiredPermission": "users:read"
    },
    {
      "path": "/api/users",
      "methods": ["POST"],
      "requiredPermission": "users:write"
    },
    {
      "path": "/api/users/{id}",
      "methods": ["GET"],
      "regex": "^/api/users/\\d+$",
      "requiredPermission": "users:read"
    },
    {
      "path": "/api/users/{id}",
      "methods": ["PUT", "PATCH"],
      "regex": "^/api/users/\\d+$",
      "requiredPermission": "users:write"
    },
    {
      "path": "/api/users/{id}",
      "methods": ["DELETE"],
      "regex": "^/api/users/\\d+$",
      "requiredPermission": "users:delete",
      "allowedRoles": ["admin"]
    }
  ]
}
```

## Usage

```go
import (
    "gofr.dev/pkg/gofr"
    "gofr.dev/pkg/gofr/rbac"
)

func main() {
    app := gofr.New()
    
    // Create RBAC provider (follows DBResolver pattern)
    provider := rbac.NewProvider(app, "configs/rbac.json", &rbac.JWTExtractor{Claim: "role"})
    
    // Add RBAC - all authorization handled by middleware based on config
    // Uses unified Roles + Endpoints format only
    app.AddRBAC(provider)
    
    // Define REST API routes - authorization is automatic based on config
    app.GET("/api/users", getAllUsers)        // Auto-checked: users:read
    app.POST("/api/users", createUser)        // Auto-checked: users:write
    app.DELETE("/api/users/:id", deleteUser) // Auto-checked: users:delete
    
    app.Run()
}
```

## Summary

✅ **Single Way to Declare RBAC**: Configuration files only using `Roles` + `Endpoints` format  
✅ **Role-Based with Attributes**: Roles assigned to attributes (ABAC support)  
✅ **REST API Endpoint Mapping**: Follows industry best practices  
✅ **No REST API for Management**: RBAC is infrastructure, not application data  
✅ **No Programmatic Functions**: All authorization through config  
✅ **No Deprecated Fields**: All legacy fields (`route`, `overrides`, `roleHierarchy`, `permissions`) removed  
✅ **Industry Standard**: Follows AWS IAM, Google Cloud IAM patterns  

This ensures consistency, auditability, and maintainability across all GoFr applications.

