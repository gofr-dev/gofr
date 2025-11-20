# Advanced RBAC Example

This example demonstrates advanced RBAC features including hot-reload, custom error handling, audit logging, and caching.

## Features Demonstrated

- Hot-reload configuration
- Custom error handlers
- Custom audit logging
- Role caching
- YAML configuration
- Role hierarchy
- Permission-based access control

## Running the Example

1. Start the application:
```bash
go run main.go
```

2. Test endpoints:
```bash
# As admin
curl -H "X-User-Role: admin" http://localhost:8000/api/users
curl -H "X-User-Role: admin" http://localhost:8000/api/admin

# As editor (inherits viewer permissions via hierarchy)
curl -H "X-User-Role: editor" http://localhost:8000/api/users
```

3. Modify `configs/rbac.yaml` and watch it reload automatically (every 30 seconds)

## Configuration Files

- `configs/rbac.yaml` - YAML configuration with all features
- `configs/rbac.json` - JSON configuration (alternative)

## Features

### Hot-Reload
Configuration automatically reloads when the file changes (every 30 seconds).

### Custom Error Handler
Returns JSON error responses instead of plain text.

### Custom Audit Logger
Logs all authorization decisions to GoFr logger.

### Caching
Role lookups are cached for 5 minutes to improve performance.

### Role Hierarchy
- Admin inherits: editor, author, viewer
- Editor inherits: author, viewer
- Author inherits: viewer

### Permissions
Fine-grained permission-based access control with route mapping.

