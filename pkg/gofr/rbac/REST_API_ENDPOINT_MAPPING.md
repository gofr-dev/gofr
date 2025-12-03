# REST API Endpoint Mapping Guide

This guide explains how to map REST API endpoints to RBAC permissions and roles, following industry best practices from AWS API Gateway, Google Cloud Endpoints, and other major platforms.

## Overview

GoFr RBAC uses a **unified configuration format** that maps REST API endpoints to authorization requirements. This is the **only way** to declare RBAC - all authorization rules are defined in configuration files.

## REST API Design Principles

Based on industry best practices (AWS, Google, Microsoft):

### 1. Resource-Based URLs
REST APIs use resource-based URLs:
- `/api/users` - Collection of users
- `/api/users/{id}` - Specific user
- `/api/posts` - Collection of posts
- `/api/posts/{id}/comments` - Nested resource

### 2. HTTP Methods Map to Actions
Standard REST mapping:
- `GET` → Read (list or retrieve)
- `POST` → Create
- `PUT` → Update (full replacement)
- `PATCH` → Partial update
- `DELETE` → Delete

### 3. Permission Naming Convention
Follow the pattern: `resource:action`
- `users:read` - Read users
- `users:write` - Create/update users
- `users:delete` - Delete users
- `posts:read` - Read posts
- `posts:write` - Create/update posts

## Endpoint Mapping Strategy

### Recommended: Permission-Based Mapping

Map endpoints to permissions, then assign permissions to roles. This follows AWS IAM and Google Cloud IAM patterns.

```json
{
  "roles": [
    {
      "name": "admin",
      "permissions": ["*:*"]
    },
    {
      "name": "editor",
      "permissions": ["users:read", "users:write", "posts:read", "posts:write"]
    },
    {
      "name": "viewer",
      "permissions": ["users:read", "posts:read"]
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
      "requiredPermission": "users:delete"
    }
  ]
}
```

### Alternative: Direct Role Mapping

For simpler use cases, map endpoints directly to roles:

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
    },
    {
      "path": "/api/users",
      "methods": ["POST", "PUT"],
      "allowedRoles": ["admin", "editor"]
    }
  ]
}
```

### Combined: Permission + Role

Use both for maximum security (both checks must pass):

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

## Complete REST API Example

Following RESTful design principles:

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
        "posts:write",
        "comments:read",
        "comments:write"
      ],
      "attributes": {
        "department": ["engineering", "content"]
      },
      "inheritsFrom": ["viewer"]
    },
    {
      "name": "viewer",
      "permissions": [
        "users:read",
        "posts:read",
        "comments:read"
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
      "path": "/metrics",
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
    },
    {
      "path": "/api/posts",
      "methods": ["GET"],
      "requiredPermission": "posts:read"
    },
    {
      "path": "/api/posts",
      "methods": ["POST"],
      "requiredPermission": "posts:write"
    },
    {
      "path": "/api/posts/{id}",
      "methods": ["GET"],
      "regex": "^/api/posts/\\d+$",
      "requiredPermission": "posts:read"
    },
    {
      "path": "/api/posts/{id}",
      "methods": ["PUT", "PATCH"],
      "regex": "^/api/posts/\\d+$",
      "requiredPermission": "posts:write"
    },
    {
      "path": "/api/posts/{id}",
      "methods": ["DELETE"],
      "regex": "^/api/posts/\\d+$",
      "requiredPermission": "posts:delete"
    },
    {
      "path": "/api/posts/{id}/comments",
      "methods": ["GET"],
      "regex": "^/api/posts/\\d+/comments$",
      "requiredPermission": "comments:read"
    },
    {
      "path": "/api/posts/{id}/comments",
      "methods": ["POST"],
      "regex": "^/api/posts/\\d+/comments$",
      "requiredPermission": "comments:write"
    },
    {
      "path": "/api/admin/*",
      "methods": ["*"],
      "allowedRoles": ["admin"]
    }
  ]
}
```

## Best Practices

### 1. Use Permission-Based Mapping (Recommended)

**Why:**
- Fine-grained control
- Easy to audit
- Follows principle of least privilege
- Matches AWS IAM, Google Cloud IAM patterns
- Easier to manage as API grows

**Example:**
```json
{
  "endpoints": [
    {
      "path": "/api/users",
      "methods": ["GET"],
      "requiredPermission": "users:read"
    }
  ]
}
```

### 2. Map HTTP Methods to Actions

Follow REST conventions:
- `GET /api/users` → `users:read`
- `POST /api/users` → `users:write`
- `PUT /api/users/{id}` → `users:write`
- `DELETE /api/users/{id}` → `users:delete`

### 3. Use Regex for Dynamic Paths

For paths with parameters:
```json
{
  "regex": "^/api/users/\\d+$",
  "methods": ["GET"],
  "requiredPermission": "users:read"
}
```

### 4. Define Roles with Permissions

Roles should define what they can do:
```json
{
  "name": "editor",
  "permissions": ["users:read", "users:write", "posts:read", "posts:write"]
}
```

### 5. Use Role Attributes for ABAC

Enable attribute-based access control:
```json
{
  "name": "editor",
  "permissions": ["users:write"],
  "attributes": {
    "department": ["engineering", "content"],
    "region": ["us-east", "eu-west"]
  }
}
```

### 6. Leverage Role Inheritance

Reduce duplication:
```json
{
  "name": "editor",
  "permissions": ["users:write"],
  "inheritsFrom": ["viewer"]
}
```

## Comparison with Industry Standards

### AWS API Gateway + IAM
- **Permissions**: Resource-based policies (`arn:aws:execute-api:...`)
- **Roles**: IAM roles with policies
- **Mapping**: API Gateway authorizers map to IAM permissions
- **GoFr Equivalent**: `endpoints` → `requiredPermission` → `roles.permissions`

### Google Cloud Endpoints + IAM
- **Permissions**: Service-level permissions (`service.resource.action`)
- **Roles**: IAM roles (viewer, editor, owner)
- **Mapping**: OpenAPI spec with security requirements
- **GoFr Equivalent**: `endpoints` → `requiredPermission` → `roles.permissions`

### Microsoft Azure API Management
- **Policies**: XML-based policies for authorization
- **Roles**: Azure AD roles
- **Mapping**: Policy expressions map to roles
- **GoFr Equivalent**: `endpoints` → `allowedRoles`

## Single Way to Declare RBAC

**GoFr RBAC uses only configuration files** - there are no programmatic functions. All authorization is declared in:

1. **Roles** - Define roles with permissions and attributes
2. **Endpoints** - Map REST API endpoints to permissions/roles

This ensures:
- ✅ Single source of truth
- ✅ Easy to audit
- ✅ Version control friendly
- ✅ No code changes needed for authorization updates
- ✅ Consistent with infrastructure-as-code practices

## Example: Complete REST API

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
      "permissions": ["users:read", "users:write", "posts:read", "posts:write"],
      "inheritsFrom": ["viewer"]
    },
    {
      "name": "viewer",
      "permissions": ["users:read", "posts:read"]
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

This configuration provides complete REST API authorization with a single, unified format.

