# Unified RBAC Principles

## Single Way to Declare RBAC

GoFr RBAC uses **only configuration files** - there are no programmatic functions, no REST API endpoints for management, and no code-based authorization rules. This ensures:

- ✅ **Single Source of Truth**: All authorization rules in one place
- ✅ **Version Control**: RBAC changes tracked in git
- ✅ **Audit Trail**: Complete history of authorization changes
- ✅ **Infrastructure as Code**: RBAC configuration is declarative
- ✅ **No Code Changes**: Update authorization without redeploying
- ✅ **Consistency**: Same pattern across all environments

## Configuration Format

### Roles with Attributes (RBAC + ABAC)

Roles define what they can do and under what conditions:

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
      },
      "inheritsFrom": ["viewer"]
    }
  ]
}
```

**Key Points:**
- **Permissions**: What the role can do (format: `resource:action`)
- **Attributes**: Conditions/constraints for the role (enables ABAC)
- **InheritsFrom**: Roles this role inherits permissions from

### Endpoint Mapping (REST API Design)

Endpoints map REST API routes to authorization requirements:

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
      "methods": ["DELETE"],
      "regex": "^/api/users/\\d+$",
      "requiredPermission": "users:delete",
      "allowedRoles": ["admin"]
    }
  ]
}
```

**Key Points:**
- **Methods**: HTTP methods (GET, POST, PUT, DELETE, PATCH)
- **Path**: Route pattern (supports wildcards)
- **Regex**: Regular expression for dynamic paths
- **RequiredPermission**: Permission required (recommended)
- **AllowedRoles**: Direct role mapping (alternative)
- **Public**: Public endpoint (bypasses auth)

## Industry Best Practices

### AWS IAM Pattern
- **Roles**: IAM roles with policies
- **Permissions**: Resource-based (`arn:aws:execute-api:...`)
- **Mapping**: API Gateway authorizers
- **GoFr Equivalent**: `roles.permissions` → `endpoints.requiredPermission`

### Google Cloud IAM Pattern
- **Roles**: IAM roles (viewer, editor, owner)
- **Permissions**: Service-level (`service.resource.action`)
- **Mapping**: OpenAPI security requirements
- **GoFr Equivalent**: `roles.permissions` → `endpoints.requiredPermission`

### Microsoft Azure Pattern
- **Roles**: Azure AD roles
- **Policies**: XML-based authorization policies
- **Mapping**: Policy expressions
- **GoFr Equivalent**: `endpoints.allowedRoles`

## Endpoint Mapping Strategy

### 1. Permission-Based (Recommended)

**Best for:**
- Fine-grained control
- Large APIs
- Complex permission models
- Compliance requirements

**Pattern:**
```json
{
  "roles": [
    {"name": "editor", "permissions": ["users:read", "users:write"]}
  ],
  "endpoints": [
    {"path": "/api/users", "methods": ["GET"], "requiredPermission": "users:read"},
    {"path": "/api/users", "methods": ["POST"], "requiredPermission": "users:write"}
  ]
}
```

### 2. Role-Based

**Best for:**
- Simple APIs
- Small teams
- Quick setup

**Pattern:**
```json
{
  "endpoints": [
    {"path": "/api/admin/*", "methods": ["*"], "allowedRoles": ["admin"]},
    {"path": "/api/users", "methods": ["GET"], "allowedRoles": ["admin", "editor", "viewer"]}
  ]
}
```

### 3. Combined (Permission + Role)

**Best for:**
- Maximum security
- Defense in depth
- Critical operations

**Pattern:**
```json
{
  "endpoints": [
    {
      "path": "/api/users/{id}",
      "methods": ["DELETE"],
      "requiredPermission": "users:delete",
      "allowedRoles": ["admin"]
    }
  ]
}
```

## REST API Method Mapping

Follow standard REST conventions:

| HTTP Method | Action | Permission Pattern | Example |
|------------|--------|-------------------|---------|
| GET | Read | `resource:read` | `users:read` |
| POST | Create | `resource:write` | `users:write` |
| PUT | Update (full) | `resource:write` | `users:write` |
| PATCH | Update (partial) | `resource:write` | `users:write` |
| DELETE | Delete | `resource:delete` | `users:delete` |

## Role Attributes (ABAC)

Roles can have attributes that enable attribute-based access control:

```json
{
  "name": "editor",
  "permissions": ["users:write"],
  "attributes": {
    "department": ["engineering", "content"],
    "region": ["us-east"],
    "environment": ["production", "staging"],
    "custom": {
      "project": ["project-a", "project-b"]
    }
  }
}
```

**Use Cases:**
- Department-based access
- Regional restrictions
- Environment-specific permissions
- Project-based access
- Time-based access (future)

## Complete Example

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
      "attributes": {
        "department": ["engineering", "content"]
      },
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
      "methods": ["DELETE"],
      "regex": "^/api/users/\\d+$",
      "requiredPermission": "users:delete",
      "allowedRoles": ["admin"]
    }
  ]
}
```

## Why Configuration-Only?

1. **No REST API for RBAC Management**: RBAC is infrastructure, not application data
2. **No Programmatic Functions**: Prevents inconsistent authorization logic
3. **Single Source of Truth**: All rules in config files
4. **Version Control**: Track all authorization changes
5. **Environment Parity**: Same config across dev/staging/prod
6. **Audit Compliance**: Complete history of who changed what

This approach follows the same principles as:
- Kubernetes RBAC (YAML configs)
- AWS IAM (JSON policies)
- Terraform (HCL configs)
- Infrastructure as Code

