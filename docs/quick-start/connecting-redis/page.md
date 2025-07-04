# Connecting to Redis

GoFr simplifies the process of connecting to Redis.

## Setup:

Ensure we have Redis installed on our system.

Optionally, we can use Docker to set up a development environment with password authentication as described below.

```bash
docker run --name gofr-redis -p 2002:6379 -d \
	-e REDIS_PASSWORD=password \
	redis:7.0.5 --requirepass password
```

We can set a sample key `greeting` using the following command:

```bash
docker exec -it gofr-redis bash -c 'redis-cli SET greeting "Hello from Redis."'
```

## Configuration & Usage:

GoFr applications rely on environment variables to configure and connect to a Redis server.  
These variables are stored in a `.env` file located within the `configs` directory at your project root.

### Required Environment Variables:

{% table %}

- Key
- Description

---

- REDIS_HOST
- Hostname or IP address of your Redis server

---

- REDIS_PORT
- Port number your Redis server listens on (default: `6379`)

---

- REDIS_USER
- Redis username; multiple users with ACLs can be configured. [See official docs](https://redis.io/docs/latest/operate/oss_and_stack/management/security/acl/)

---

- REDIS_PASSWORD
- Redis password (required only if authentication is enabled)

---

- REDIS_DB
- Redis database number (default: `0`)

---

## TLS Support (Optional):

{% table %}

- Key
- Description

---

- REDIS_TLS_ENABLED
- Set to `"true"` to enable TLS

---

- REDIS_TLS_CA_CERT_PATH
- File path to the CA certificate used to verify the Redis server

---

- REDIS_TLS_CERT_PATH
- File path to the client certificate (for mTLS)

---

- REDIS_TLS_KEY_PATH
- File path to the client private key (for mTLS)

---

## ✅ Example `.env` File

```env
REDIS_HOST=redis.example.com
REDIS_PORT=6379
REDIS_USER=appuser
REDIS_PASSWORD=securepassword
REDIS_DB=0

# TLS settings (optional)
REDIS_TLS_ENABLED=true
REDIS_TLS_CA_CERT_PATH=./configs/certs/ca.pem
REDIS_TLS_CERT_PATH=./configs/certs/client.crt
REDIS_TLS_KEY_PATH=./configs/certs/client.key
```

The following code snippet demonstrates how to retrieve data from a Redis key named "greeting":

```go
package main

import (
	"errors"

	"github.com/redis/go-redis/v9"

	"gofr.dev/pkg/gofr"
)

func main() {
	// Initialize GoFr object
	app := gofr.New()

	app.GET("/redis", func(ctx *gofr.Context) (any, error) {
		// Get the value using the Redis instance

		val, err := ctx.Redis.Get(ctx.Context, "greeting").Result()
		if err != nil && !errors.Is(err, redis.Nil) {
			// If the key is not found, we are not considering this an error and returning ""
			return nil, err
		}

		return val, nil
	})

	// Run the application

	app.Run()
}
```
| `REDIS_PASSWORD` | Redis password | - | `password` |
| `REDIS_DB` | Redis database number | 0 | `0` |

### Sample .env File

```env
# Redis Configuration
REDIS_HOST=localhost
REDIS_PORT=2002
REDIS_USER=default
REDIS_PASSWORD=password
REDIS_DB=0

# Optional: Connection Pool Settings
REDIS_MAX_IDLE_CONNS=10
REDIS_MAX_ACTIVE_CONNS=20
REDIS_IDLE_TIMEOUT=240s
REDIS_DIAL_TIMEOUT=10s
REDIS_READ_TIMEOUT=30s
REDIS_WRITE_TIMEOUT=30s
```

## Redis Interface

GoFr provides a comprehensive Redis interface with full method definitions for all common operations:

### Core Interface Methods

```go
type Redis interface {
    // Basic Key-Value Operations
    Set(key string, value interface{}, expiration time.Duration) *redis.StatusCmd
    Get(key string) *redis.StringCmd
    GetSet(key string, value interface{}) *redis.StringCmd
    Del(keys ...string) *redis.IntCmd
    Exists(keys ...string) *redis.IntCmd
    Expire(key string, expiration time.Duration) *redis.BoolCmd
    ExpireAt(key string, tm time.Time) *redis.BoolCmd
    TTL(key string) *redis.DurationCmd
    Type(key string) *redis.StatusCmd
    
    // Hash Operations
    HSet(key string, values ...interface{}) *redis.IntCmd
    HGet(key, field string) *redis.StringCmd
    HGetAll(key string) *redis.StringStringMapCmd
    HMSet(key string, values ...interface{}) *redis.BoolCmd
    HMGet(key string, fields ...string) *redis.SliceCmd
    HDel(key string, fields ...string) *redis.IntCmd
    HExists(key, field string) *redis.BoolCmd
    HKeys(key string) *redis.StringSliceCmd
    HVals(key string) *redis.StringSliceCmd
    HLen(key string) *redis.IntCmd
    HIncrBy(key, field string, incr int64) *redis.IntCmd
    
    // List Operations
    LPush(key string, values ...interface{}) *redis.IntCmd
    RPush(key string, values ...interface{}) *redis.IntCmd
    LPop(key string) *redis.StringCmd
    RPop(key string) *redis.StringCmd
    LLen(key string) *redis.IntCmd
    LRange(key string, start, stop int64) *redis.StringSliceCmd
    LIndex(key string, index int64) *redis.StringCmd
    LSet(key string, index int64, value interface{}) *redis.StatusCmd
    LRem(key string, count int64, value interface{}) *redis.IntCmd
    LTrim(key string, start, stop int64) *redis.StatusCmd
    
    // Set Operations
    SAdd(key string, members ...interface{}) *redis.IntCmd
    SMembers(key string) *redis.StringSliceCmd
    SCard(key string) *redis.IntCmd
    SIsMember(key string, member interface{}) *redis.BoolCmd
    SRem(key string, members ...interface{}) *redis.IntCmd
    SPop(key string) *redis.StringCmd
    SRandMember(key string) *redis.StringCmd
    
    // Sorted Set Operations
    ZAdd(key string, members ...redis.Z) *redis.IntCmd
    ZRange(key string, start, stop int64) *redis.StringSliceCmd
    ZRangeWithScores(key string, start, stop int64) *redis.ZSliceCmd
    ZCard(key string) *redis.IntCmd
    ZCount(key, min, max string) *redis.IntCmd
    ZRem(key string, members ...interface{}) *redis.IntCmd
    ZScore(key, member string) *redis.FloatCmd
    
    // Pub/Sub Operations
    Publish(channel string, message interface{}) *redis.IntCmd
    Subscribe(channels ...string) *redis.PubSub
    PSubscribe(patterns ...string) *redis.PubSub
    
    // Transaction Operations
    TxPipeline() redis.Pipeliner
    Pipeline() redis.Pipeliner
    
    // Utility Operations
    Ping() *redis.StatusCmd
    Info(section ...string) *redis.StringCmd
    FlushDB() *redis.StatusCmd
    FlushAll() *redis.StatusCmd
    Keys(pattern string) *redis.StringSliceCmd
    Scan(cursor uint64, match string, count int64) *redis.ScanCmd
}
```

## Full Usage Examples

### Basic GET and SET Operations

```go
package main

import (
    "time"
    "gofr.dev/pkg/gofr"
)

func main() {
    app := gofr.New()
    
    // SET operation
    app.POST("/cache/{key}", func(ctx *gofr.Context) (interface{}, error) {
        key := ctx.PathParam("key")
        value := ctx.Body()
        
        result := ctx.Redis.Set(key, value, 24*time.Hour)
        if result.Err() != nil {
            return nil, result.Err()
        }
        
        return map[string]string{"status": "success", "key": key}, nil
    })
    
    // GET operation
    app.GET("/cache/{key}", func(ctx *gofr.Context) (interface{}, error) {
        key := ctx.PathParam("key")
        
        result := ctx.Redis.Get(key)
        if result.Err() != nil {
            return nil, result.Err()
        }
        
        return map[string]string{"key": key, "value": result.Val()}, nil
    })
    
    app.Run()
}
```

### Hash Operations (User Profiles)

```go
package main

import (
    "encoding/json"
    "gofr.dev/pkg/gofr"
)

type User struct {
    ID       int    `json:"id"`
    Name     string `json:"name"`
    Email    string `json:"email"`
    Role     string `json:"role"`
}

func main() {
    app := gofr.New()
    
    // Store user profile using HSET
    app.POST("/users/{id}", func(ctx *gofr.Context) (interface{}, error) {
        userID := ctx.PathParam("id")
        var user User
        
        if err := json.Unmarshal(ctx.Body(), &user); err != nil {
            return nil, err
        }
        
        key := "user:" + userID
        result := ctx.Redis.HSet(key, 
            "id", user.ID,
            "name", user.Name,
            "email", user.Email,
            "role", user.Role,
        )
        
        if result.Err() != nil {
            return nil, result.Err()
        }
        
        return map[string]interface{}{
            "message": "User profile stored successfully",
            "user_id": userID,
        }, nil
    })
    
    // Get user profile using HGETALL
    app.GET("/users/{id}", func(ctx *gofr.Context) (interface{}, error) {
        userID := ctx.PathParam("id")
        key := "user:" + userID
        
        result := ctx.Redis.HGetAll(key)
        if result.Err() != nil {
            return nil, result.Err()
        }
        
        profile := result.Val()
        if len(profile) == 0 {
            return nil, gofr.NewError(404, "User not found")
        }
        
        return profile, nil
    })
    
    app.Run()
}
```

### List Operations (Activity Feed)

```go
package main

import (
    "encoding/json"
    "strconv"
    "time"
    "gofr.dev/pkg/gofr"
)

type Activity struct {
    UserID    int       `json:"user_id"`
    Action    string    `json:"action"`
    Timestamp time.Time `json:"timestamp"`
    Details   string    `json:"details"`
}

func main() {
    app := gofr.New()
    
    // Add activity to feed
    app.POST("/activities/{user_id}", func(ctx *gofr.Context) (interface{}, error) {
        userID := ctx.PathParam("user_id")
        var activity Activity
        
        if err := json.Unmarshal(ctx.Body(), &activity); err != nil {
            return nil, err
        }
        
        activity.Timestamp = time.Now()
        activityJSON, _ := json.Marshal(activity)
        
        key := "activities:" + userID
        result := ctx.Redis.LPush(key, activityJSON)
        
        if result.Err() != nil {
            return nil, result.Err()
        }
        
        // Keep only last 100 activities
        ctx.Redis.LTrim(key, 0, 99)
        
        return map[string]interface{}{
            "message": "Activity added successfully",
            "total_activities": result.Val(),
        }, nil
    })
    
    // Get recent activities
    app.GET("/activities/{user_id}", func(ctx *gofr.Context) (interface{}, error) {
        userID := ctx.PathParam("user_id")
        limitStr := ctx.Param("limit")
        
        limit, err := strconv.ParseInt(limitStr, 10, 64)
        if err != nil || limit <= 0 {
            limit = 10
        }
        
        key := "activities:" + userID
        result := ctx.Redis.LRange(key, 0, limit-1)
        
        if result.Err() != nil {
            return nil, result.Err()
        }
        
        activities := make([]Activity, 0, len(result.Val()))
        for _, activityJSON := range result.Val() {
            var activity Activity
            if err := json.Unmarshal([]byte(activityJSON), &activity); err == nil {
                activities = append(activities, activity)
            }
        }
        
        return activities, nil
    })
    
    app.Run()
}
```

## Common Use Cases

### 1. Caching Implementation

```go
package main

import (
    "encoding/json"
    "time"
    "gofr.dev/pkg/gofr"
)

type Product struct {
    ID          int     `json:"id"`
    Name        string  `json:"name"`
    Description string  `json:"description"`
    Price       float64 `json:"price"`
}

func main() {
    app := gofr.New()
    
    // Cache-aside pattern for product data
    app.GET("/products/{id}", func(ctx *gofr.Context) (interface{}, error) {
        productID := ctx.PathParam("id")
        cacheKey := "product:" + productID
        
        // Try to get from cache first
        cached := ctx.Redis.Get(cacheKey)
        if cached.Err() == nil {
            var product Product
            if err := json.Unmarshal([]byte(cached.Val()), &product); err == nil {
                return product, nil
            }
        }
        
        // Cache miss - fetch from database
        product, err := fetchProductFromDB(ctx, productID)
        if err != nil {
            return nil, err
        }
        
        // Store in cache for 1 hour
        productJSON, _ := json.Marshal(product)
        ctx.Redis.Set(cacheKey, productJSON, time.Hour)
        
        return product, nil
    })
    
    app.Run()
}

func fetchProductFromDB(ctx *gofr.Context, productID string) (Product, error) {
    // Simulate database fetch
    var product Product
    row := ctx.DB.QueryRow("SELECT id, name, description, price FROM products WHERE id = ?", productID)
    err := row.Scan(&product.ID, &product.Name, &product.Description, &product.Price)
    return product, err
}
```

### 2. Session Management

```go
package main

import (
    "crypto/rand"
    "encoding/hex"
    "encoding/json"
    "time"
    "gofr.dev/pkg/gofr"
)

type Session struct {
    UserID    int       `json:"user_id"`
    Username  string    `json:"username"`
    Role      string    `json:"role"`
    CreatedAt time.Time `json:"created_at"`
    LastSeen  time.Time `json:"last_seen"`
}

func main() {
    app := gofr.New()
    
    // Create session
    app.POST("/auth/login", func(ctx *gofr.Context) (interface{}, error) {
        // Authenticate user (implementation not shown)
        userID := 123
        username := "john_doe"
        role := "user"
        
        // Generate session token
        sessionToken := generateSessionToken()
        
        session := Session{
            UserID:    userID,
            Username:  username,
            Role:      role,
            CreatedAt: time.Now(),
            LastSeen:  time.Now(),
        }
        
        sessionJSON, _ := json.Marshal(session)
        sessionKey := "session:" + sessionToken
        
        // Store session with 24-hour expiration
        result := ctx.Redis.Set(sessionKey, sessionJSON, 24*time.Hour)
        if result.Err() != nil {
            return nil, result.Err()
        }
        
        return map[string]string{
            "session_token": sessionToken,
            "message": "Login successful",
        }, nil
    })
    
    // Validate session middleware
    app.GET("/protected", func(ctx *gofr.Context) (interface{}, error) {
        sessionToken := ctx.Header("Authorization")
        if sessionToken == "" {
            return nil, gofr.NewError(401, "Missing session token")
        }
        
        sessionKey := "session:" + sessionToken
        result := ctx.Redis.Get(sessionKey)
        
        if result.Err() != nil {
            return nil, gofr.NewError(401, "Invalid session")
        }
        
        var session Session
        if err := json.Unmarshal([]byte(result.Val()), &session); err != nil {
            return nil, gofr.NewError(401, "Invalid session format")
        }
        
        // Update last seen
        session.LastSeen = time.Now()
        sessionJSON, _ := json.Marshal(session)
        ctx.Redis.Set(sessionKey, sessionJSON, 24*time.Hour)
        
        return map[string]interface{}{
            "message": "Access granted",
            "user": session,
        }, nil
    })
    
    app.Run()
}

func generateSessionToken() string {
    bytes := make([]byte, 32)
    rand.Read(bytes)
    return hex.EncodeToString(bytes)
}
```

### 3. Rate Limiting

```go
package main

import (
    "fmt"
    "strconv"
    "time"
    "gofr.dev/pkg/gofr"
)

func main() {
    app := gofr.New()
    
    // Rate limiter middleware
    rateLimiter := func(limit int, window time.Duration) gofr.Handler {
        return func(ctx *gofr.Context) (interface{}, error) {
            clientIP := ctx.Header("X-Real-IP")
            if clientIP == "" {
                clientIP = ctx.Header("X-Forwarded-For")
            }
            if clientIP == "" {
                clientIP = "unknown"
            }
            
            key := fmt.Sprintf("rate_limit:%s", clientIP)
            
            // Get current count
            result := ctx.Redis.Get(key)
            var currentCount int
            
            if result.Err() == nil {
                currentCount, _ = strconv.Atoi(result.Val())
            }
            
            if currentCount >= limit {
                return nil, gofr.NewError(429, "Rate limit exceeded")
            }
            
            // Increment counter
            pipe := ctx.Redis.Pipeline()
            pipe.Incr(key)
            pipe.Expire(key, window)
            _, err := pipe.Exec()
            
            if err != nil {
                return nil, err
            }
            
            return map[string]interface{}{
                "message": "Request processed",
                "remaining": limit - currentCount - 1,
            }, nil
        }
    }
    
    // Apply rate limiting (100 requests per hour)
    app.GET("/api/data", rateLimiter(100, time.Hour), func(ctx *gofr.Context) (interface{}, error) {
        return map[string]string{"data": "sensitive information"}, nil
    })
    
    app.Run()
}
```

## Data Migration Support

GoFr provides built-in support for Redis data migrations using the migration framework:

### Migration Example

```go
package main

import (
    "encoding/json"
    "gofr.dev/pkg/gofr"
    "gofr.dev/pkg/gofr/migration"
)

type UserMigration struct{}

func (m UserMigration) Up(ctx *gofr.Context) error {
    // Migrate user data format from v1 to v2
    keys := ctx.Redis.Keys("user:*")
    if keys.Err() != nil {
        return keys.Err()
    }
    
    for _, key := range keys.Val() {
        // Get existing user data
        userData := ctx.Redis.HGetAll(key)
        if userData.Err() != nil {
            continue
        }
        
        userMap := userData.Val()
        
        // Add new fields for v2 format
        if _, exists := userMap["created_at"]; !exists {
            ctx.Redis.HSet(key, "created_at", "2024-01-01T00:00:00Z")
        }
        
        if _, exists := userMap["updated_at"]; !exists {
            ctx.Redis.HSet(key, "updated_at", "2024-01-01T00:00:00Z")
        }
        
        if _, exists := userMap["status"]; !exists {
            ctx.Redis.HSet(key, "status", "active")
        }
        
        // Set version
        ctx.Redis.HSet(key, "version", "v2")
    }
    
    return nil
}

func (m UserMigration) Down(ctx *gofr.Context) error {
    // Rollback migration - remove v2 fields
    keys := ctx.Redis.Keys("user:*")
    if keys.Err() != nil {
        return keys.Err()
    }
    
    for _, key := range keys.Val() {
        ctx.Redis.HDel(key, "created_at", "updated_at", "status", "version")
    }
    
    return nil
}

func main() {
    app := gofr.New()
    
    // Register migration
    app.Migrate(migration.Migrate{
        20240101120000: UserMigration{},
    })
    
    app.Run()
}
```

### Advanced Migration with Data Transformation

```go
package main

import (
    "encoding/json"
    "strings"
    "gofr.dev/pkg/gofr"
    "gofr.dev/pkg/gofr/migration"
)

type ProductCategoryMigration struct{}

func (m ProductCategoryMigration) Up(ctx *gofr.Context) error {
    // Migrate product categories from flat structure to hierarchical
    keys := ctx.Redis.Keys("product:*")
    if keys.Err() != nil {
        return keys.Err()
    }
    
    for _, key := range keys.Val() {
        product := ctx.Redis.HGetAll(key)
        if product.Err() != nil {
            continue
        }
        
        productData := product.Val()
        category := productData["category"]
        
        // Transform category format
        if category != "" && !strings.Contains(category, ":") {
            // Convert "electronics" to "electronics:general"
            newCategory := category + ":general"
            ctx.Redis.HSet(key, "category", newCategory)
            ctx.Redis.HSet(key, "category_level", "2")
        }
    }
    
    return nil
}

func (m ProductCategoryMigration) Down(ctx *gofr.Context) error {
    // Rollback hierarchical categories to flat structure
    keys := ctx.Redis.Keys("product:*")
    if keys.Err() != nil {
        return keys.Err()
    }
    
    for _, key := range keys.Val() {
        product := ctx.Redis.HGetAll(key)
        if product.Err() != nil {
            continue
        }
        
        productData := product.Val()
        category := productData["category"]
        
        // Convert back to flat structure
        if strings.Contains(category, ":") {
            flatCategory := strings.Split(category, ":")[0]
            ctx.Redis.HSet(key, "category", flatCategory)
            ctx.Redis.HDel(key, "category_level")
        }
    }
    
    return nil
}

func main() {
    app := gofr.New()
    
    // Register migration
    app.Migrate(migration.Migrate{
        20240201120000: ProductCategoryMigration{},
    })
    
    app.Run()
}
```

## Error Handling and Best Practices

### Connection Error Handling

```go
package main

import (
    "errors"
    "github.com/redis/go-redis/v9"
    "gofr.dev/pkg/gofr"
)

func main() {
    app := gofr.New()
    
    app.GET("/health/redis", func(ctx *gofr.Context) (interface{}, error) {
        result := ctx.Redis.Ping()
        if result.Err() != nil {
            return map[string]interface{}{
                "status": "unhealthy",
                "error": result.Err().Error(),
            }, nil
        }
        
        return map[string]interface{}{
            "status": "healthy",
            "response": result.Val(),
        }, nil
    })
    
    // Handle Redis connection errors gracefully
    app.GET("/cache/{key}", func(ctx *gofr.Context) (interface{}, error) {
        key := ctx.PathParam("key")
        
        result := ctx.Redis.Get(key)
        if result.Err() != nil {
            if errors.Is(result.Err(), redis.Nil) {
                return map[string]interface{}{
                    "key": key,
                    "found": false,
                    "message": "Key not found",
                }, nil
            }
            
            // Log error and return graceful response
            ctx.Logger.Error("Redis error", "error", result.Err())
            return nil, gofr.NewError(500, "Cache service unavailable")
        }
        
        return map[string]interface{}{
            "key": key,
            "value": result.Val(),
            "found": true,
        }, nil
    })
    
    app.Run()
}
```

### Performance Optimization

```go
package main

import (
    "time"
    "gofr.dev/pkg/gofr"
)

func main() {
    app := gofr.New()
    
    // Use pipelining for multiple operations
    app.POST("/batch/cache", func(ctx *gofr.Context) (interface{}, error) {
        type CacheItem struct {
            Key   string `json:"key"`
            Value string `json:"value"`
            TTL   int    `json:"ttl"`
        }
        
        var items []CacheItem
        if err := ctx.Bind(&items); err != nil {
            return nil, err
        }
        
        // Use pipeline for batch operations
        pipe := ctx.Redis.Pipeline()
        
        for _, item := range items {
            ttl := time.Duration(item.TTL) * time.Second
            pipe.Set(item.Key, item.Value, ttl)
        }
        
        results, err := pipe.Exec()
        if err != nil {
            return nil, err
        }
        
        return map[string]interface{}{
            "processed": len(results),
            "message": "Batch cache operation completed",
        }, nil
    })
    
    app.Run()
}
```

## Testing with GoFr Redis

```go
package main

import (
    "testing"
    "gofr.dev/pkg/gofr"
)

func TestRedisOperations(t *testing.T) {
    app := gofr.New()
    
    // Test SET operation
    t.Run("Test SET operation", func(t *testing.T) {
        ctx := &gofr.Context{
            Redis: app.Redis,
        }
        
        result := ctx.Redis.Set("test:key", "test_value", 0)
        if result.Err() != nil {
            t.Errorf("SET operation failed: %v", result.Err())
        }
        
        // Verify the value
        getResult := ctx.Redis.Get("test:key")
        if getResult.Err() != nil {
            t.Errorf("GET operation failed: %v", getResult.Err())
        }
        
        if getResult.Val() != "test_value" {
            t.Errorf("Expected 'test_value', got '%s'", getResult.Val())
        }
    })
    
    // Test Hash operations
    t.Run("Test Hash operations", func(t *testing.T) {
        ctx := &gofr.Context{
            Redis: app.Redis,
        }
        
        // Test HSET
        result := ctx.Redis.HSet("test:hash", "field1", "value1", "field2", "value2")
        if result.Err() != nil {
            t.Errorf("HSET operation failed: %v", result.Err())
        }
        
        // Test HGETALL
        hashResult := ctx.Redis.HGetAll("test:hash")
        if hashResult.Err() != nil {
            t.Errorf("HGETALL operation failed: %v", hashResult.Err())
        }
        
        values := hashResult.Val()
        if values["field1"] != "value1" || values["field2"] != "value2" {
            t.Errorf("Hash values don't match expected values")
        }
    })
}
```