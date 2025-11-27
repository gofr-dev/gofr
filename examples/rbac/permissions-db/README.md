# Permission-Based RBAC (Database) Example

## Overview

This example demonstrates **permission-based access control** with **database-based role extraction**. Roles are fetched from a database based on the user ID, making it ideal for dynamic role management.

## Use Case

**When to use:**
- Roles stored in database and change frequently
- Multi-tenant applications with tenant-specific roles
- User roles managed through admin panels
- Applications requiring real-time role updates
- Complex role hierarchies stored in database

**Example scenarios:**
- SaaS platforms with user management dashboards
- Enterprise applications with dynamic role assignment
- Content management systems with editor workflows
- Multi-tenant applications

## How It Works

1. **User ID Extraction**: User ID extracted from request (header, JWT, session, etc.)
2. **Database Query**: Role fetched from database using user ID
3. **Permission Mapping**: Route + HTTP method mapped to required permission
4. **Permission Check**: System checks if user's role has the required permission
5. **Authorization**: Access granted/denied based on permission check

## Configuration

### Database Schema

Create a users table with role information:

```sql
CREATE TABLE users (
    id VARCHAR(255) PRIMARY KEY,
    email VARCHAR(255),
    role VARCHAR(50),
    created_at TIMESTAMP
);

-- Example data
INSERT INTO users (id, email, role) VALUES
    ('user1', 'admin@example.com', 'admin'),
    ('user2', 'editor@example.com', 'editor'),
    ('user3', 'viewer@example.com', 'viewer');
```

### RBAC Config (`configs/rbac.json`)

```json
{
  "route": {
    "/api/*": ["admin", "editor"]
  },
  "enablePermissions": true
}
```

### Permission Configuration (in code)

```go
config.PermissionConfig = &rbac.PermissionConfig{
    Permissions: map[string][]string{
        "users:read":   {"admin", "editor", "viewer"},
        "users:write":  {"admin", "editor"},
        "users:delete": {"admin"},
    },
    RoutePermissionMap: map[string]string{
        "GET /api/users":    "users:read",
        "POST /api/users":   "users:write",
        "DELETE /api/users": "users:delete",
    },
}

// CRITICAL: Set RequiresContainer = true for database-based role extraction
// This enables container access in RoleExtractorFunc
config.RequiresContainer = true
```

## Setup Instructions

### 1. Database Configuration

Configure your database connection in `.env` or `configs/.env`:

```env
DB_HOST=localhost
DB_PORT=3306
DB_USER=root
DB_PASSWORD=password
DB_NAME=testdb
DB_DIALECT=mysql
```

### 2. Create Database Schema

```sql
CREATE DATABASE testdb;
USE testdb;

CREATE TABLE users (
    id VARCHAR(255) PRIMARY KEY,
    email VARCHAR(255),
    role VARCHAR(50)
);

INSERT INTO users (id, email, role) VALUES
    ('user1', 'admin@example.com', 'admin'),
    ('user2', 'editor@example.com', 'editor'),
    ('user3', 'viewer@example.com', 'viewer');
```

### 3. Start the Application

```bash
go run main.go
```

### 4. Test with Different Users

```bash
# Admin user - can read, write, and delete
curl -H "X-User-ID: user1" http://localhost:8000/api/users
curl -X POST -H "X-User-ID: user1" http://localhost:8000/api/users
curl -X DELETE -H "X-User-ID: user1" http://localhost:8000/api/users

# Editor user - can read and write, but not delete
curl -H "X-User-ID: user2" http://localhost:8000/api/users
curl -X POST -H "X-User-ID: user2" http://localhost:8000/api/users
curl -X DELETE -H "X-User-ID: user2" http://localhost:8000/api/users  # Should fail

# Viewer user - can only read
curl -H "X-User-ID: user3" http://localhost:8000/api/users
curl -X POST -H "X-User-ID: user3" http://localhost:8000/api/users  # Should fail
```

## API Endpoints

- `GET /api/users` - Requires: `users:read` permission
- `POST /api/users` - Requires: `users:write` permission
- `DELETE /api/users` - Requires: `users:delete` permission
- `GET /api/posts` - Requires: `posts:read` permission
- `POST /api/posts` - Requires: `posts:write` permission

## User ID Extraction Options

### From Header
```go
userID := req.Header.Get("X-User-ID")
```

### From JWT Token
```go
// After OAuth middleware validates JWT
claims := req.Context().Value(middleware.JWTClaim).(jwt.MapClaims)
userID := claims["sub"].(string)
```

### From Session
```go
session := req.Context().Value("session")
userID := session.UserID
```

## Performance Considerations

⚠️ **Database queries on every request can be slow!**

**Optimization strategies:**

1. **Implement Application-Level Caching:**
   - Use Redis/Memcached for role caching
   - Implement TTL-based cache invalidation
   - Cache roles with appropriate expiration times

2. **Use Connection Pooling:**
   - GoFr automatically manages database connection pools
   - Configure pool size in database config

3. **Batch Queries:**
   - If extracting multiple user roles, batch the queries

## Advantages

✅ **Dynamic Roles**: Roles can be updated in database without code changes  
✅ **Multi-Tenant**: Easy to implement tenant-specific roles  
✅ **Admin-Managed**: Roles can be managed through admin interfaces  
✅ **Flexible**: Supports complex role hierarchies stored in database

## Limitations

⚠️ **Performance**: Database query on every request (mitigate with caching)  
⚠️ **Latency**: Additional database round-trip adds latency  
⚠️ **Database Dependency**: Application depends on database availability

## Best Practices

1. **Implement application-level caching** (e.g., Redis) for production applications to reduce database load
2. **Use connection pooling** to manage database connections
3. **Handle database errors gracefully** (fallback to default role or deny access)
4. **Monitor query performance** and optimize slow queries
5. **Consider read replicas** for role lookups to reduce load on primary DB

## Example: With Application-Level Caching

For production applications, implement caching at the application level using Redis or similar:

```go
// CRITICAL: Set RequiresContainer = true for database-based role extraction
config.RequiresContainer = true

// Role extraction with application-level caching
// Container is passed as args[0] when RequiresContainer = true
config.RoleExtractorFunc = func(req *http.Request, args ...any) (string, error) {
    userID := req.Header.Get("X-User-ID")
    
    // Get container from args (only available when RequiresContainer = true)
    if len(args) > 0 {
        if cntr, ok := args[0].(*container.Container); ok && cntr != nil {
            // Check Redis cache first
            if cntr.Redis != nil {
                cachedRole, err := cntr.Redis.Get(req.Context(), "rbac:role:"+userID).Result()
                if err == nil {
                    return cachedRole, nil
                }
            }
            
            // Cache miss - query database
            if cntr.SQL != nil {
                var role string
                err := cntr.SQL.QueryRowContext(req.Context(), "SELECT role FROM users WHERE id = ?", userID).Scan(&role)
                if err != nil {
                    return "", err
                }
                
                // Cache the role for future requests (5 minute TTL)
                if cntr.Redis != nil {
                    _ = cntr.Redis.Set(req.Context(), "rbac:role:"+userID, role, 5*time.Minute)
                }
                
                return role, nil
            }
        }
    }
    return "", fmt.Errorf("database not available")
}
```

