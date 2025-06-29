# Redis

GoFr provides built-in support for Redis, enabling applications to leverage Redis for fast in-memory data storage, caching, and session management through `gofr.Context`.

```go
// Redis provides methods for GoFr applications to communicate with Redis
// through its commands and operations.
type Redis interface {
	// HealthChecker verifies if the Redis server is reachable.
	// Returns an error if the server is unreachable, otherwise nil.
	HealthChecker

	// Get retrieves the value of a key from Redis.
	//
	// Parameters:
	// - ctx: Context for managing request lifetime.
	// - key: The key to retrieve.
	//
	// Returns:
	// - The value associated with the key.
	// - Error if the key doesn't exist or connectivity issues occur.
	Get(ctx context.Context, key string) *redis.StringCmd

	// Set stores a key-value pair in Redis.
	//
	// Parameters:
	// - ctx: Context for managing request lifetime.
	// - key: The key to store.
	// - value: The value to associate with the key.
	// - expiration: Time duration after which the key expires (0 for no expiration).
	//
	// Returns:
	// - Error if the operation fails or connectivity issues occur.
	Set(ctx context.Context, key string, value interface{}, expiration time.Duration) *redis.StatusCmd

	// Del removes one or more keys from Redis.
	//
	// Parameters:
	// - ctx: Context for managing request lifetime.
	// - keys: Variable number of keys to delete.
	//
	// Returns:
	// - Number of keys that were removed.
	// - Error if connectivity issues occur.
	Del(ctx context.Context, keys ...string) *redis.IntCmd

	// Exists checks if one or more keys exist in Redis.
	//
	// Parameters:
	// - ctx: Context for managing request lifetime.
	// - keys: Variable number of keys to check.
	//
	// Returns:
	// - Number of keys that exist.
	// - Error if connectivity issues occur.
	Exists(ctx context.Context, keys ...string) *redis.IntCmd

	// Expire sets a timeout on a key.
	//
	// Parameters:
	// - ctx: Context for managing request lifetime.
	// - key: The key to set expiration on.
	// - expiration: Time duration after which the key expires.
	//
	// Returns:
	// - Boolean indicating if the timeout was set.
	// - Error if connectivity issues occur.
	Expire(ctx context.Context, key string, expiration time.Duration) *redis.BoolCmd

	// HGet gets the value of a hash field.
	//
	// Parameters:
	// - ctx: Context for managing request lifetime.
	// - key: The hash key.
	// - field: The field in the hash.
	//
	// Returns:
	// - The value of the hash field.
	// - Error if the field doesn't exist or connectivity issues occur.
	HGet(ctx context.Context, key, field string) *redis.StringCmd

	// HSet sets the value of a hash field.
	//
	// Parameters:
	// - ctx: Context for managing request lifetime.
	// - key: The hash key.
	// - values: Variable number of field-value pairs.
	//
	// Returns:
	// - Number of fields that were added.
	// - Error if connectivity issues occur.
	HSet(ctx context.Context, key string, values ...interface{}) *redis.IntCmd

	// HGetAll gets all fields and values in a hash.
	//
	// Parameters:
	// - ctx: Context for managing request lifetime.
	// - key: The hash key.
	//
	// Returns:
	// - Map of field-value pairs.
	// - Error if connectivity issues occur.
	HGetAll(ctx context.Context, key string) *redis.StringStringMapCmd

	// LPush prepends one or more values to a list.
	//
	// Parameters:
	// - ctx: Context for managing request lifetime.
	// - key: The list key.
	// - values: Variable number of values to prepend.
	//
	// Returns:
	// - Length of the list after the push operation.
	// - Error if connectivity issues occur.
	LPush(ctx context.Context, key string, values ...interface{}) *redis.IntCmd

	// RPop removes and returns the last element of a list.
	//
	// Parameters:
	// - ctx: Context for managing request lifetime.
	// - key: The list key.
	//
	// Returns:
	// - The removed element.
	// - Error if the list is empty or connectivity issues occur.
	RPop(ctx context.Context, key string) *redis.StringCmd

	// SAdd adds one or more members to a set.
	//
	// Parameters:
	// - ctx: Context for managing request lifetime.
	// - key: The set key.
	// - members: Variable number of members to add.
	//
	// Returns:
	// - Number of elements that were added to the set.
	// - Error if connectivity issues occur.
	SAdd(ctx context.Context, key string, members ...interface{}) *redis.IntCmd

	// SMembers gets all members in a set.
	//
	// Parameters:
	// - ctx: Context for managing request lifetime.
	// - key: The set key.
	//
	// Returns:
	// - Slice of all members in the set.
	// - Error if connectivity issues occur.
	SMembers(ctx context.Context, key string) *redis.StringSliceCmd

	// Incr increments the integer value of a key by one.
	//
	// Parameters:
	// - ctx: Context for managing request lifetime.
	// - key: The key to increment.
	//
	// Returns:
	// - The value after increment.
	// - Error if the key contains a non-integer value or connectivity issues occur.
	Incr(ctx context.Context, key string) *redis.IntCmd

	// Decr decrements the integer value of a key by one.
	//
	// Parameters:
	// - ctx: Context for managing request lifetime.
	// - key: The key to decrement.
	//
	// Returns:
	// - The value after decrement.
	// - Error if the key contains a non-integer value or connectivity issues occur.
	Decr(ctx context.Context, key string) *redis.IntCmd
}
```

## Configuration

GoFr applications rely on environment variables to configure and connect to a Redis server.
These variables are stored in a `.env` file located within the `configs` directory at your project root.

### Required Environment Variables:

| Key | Description |
|-----|-------------|
| REDIS_HOST | Hostname or IP address of your Redis server |
| REDIS_PORT | Port number your Redis server listens on (default: 6379) |
| REDIS_USER | Redis username; multiple users with ACLs can be configured |
| REDIS_PASSWORD | Password for Redis authentication |

### Example `.env` file:

```env
REDIS_HOST=localhost
REDIS_PORT=6379
REDIS_USER=default
REDIS_PASSWORD=password
```

## Setup with Docker

You can use Docker to set up a development environment with password authentication:

```bash
docker run --name gofr-redis -p 2002:6379 -d \
  -e REDIS_PASSWORD=password \
  redis:7.0.5 --requirepass password
```

Set a sample key for testing:

```bash
docker exec -it gofr-redis bash -c 'redis-cli SET greeting "Hello from Redis."'
```

## Usage Example

The following example demonstrates using Redis in a GoFr application for basic key-value operations, hash operations, and list operations.

```go
package main

import (
	"context"
	"time"

	"gofr.dev/pkg/gofr"
)

func main() {
	app := gofr.New()

	// Register routes
	app.GET("/health", redisHealthCheck)
	app.POST("/set/:key", setKey)
	app.GET("/get/:key", getKey)
	app.DELETE("/delete/:key", deleteKey)
	app.POST("/hash/:key", setHash)
	app.GET("/hash/:key", getHash)
	app.POST("/list/:key", pushToList)
	app.GET("/list/:key", popFromList)
	app.POST("/counter/:key", incrementCounter)

	// Run the app
	app.Run()
}

// Health check for Redis
func redisHealthCheck(c *gofr.Context) (any, error) {
	// Redis health check is automatically handled by GoFr
	// You can also manually ping Redis
	result := c.Redis.Ping(context.Background())
	if result.Err() != nil {
		return nil, result.Err()
	}
	return map[string]string{"status": "Redis is healthy", "ping": result.Val()}, nil
}

// Set a key-value pair with optional expiration
func setKey(c *gofr.Context) (any, error) {
	key := c.PathParam("key")
	
	var req struct {
		Value      string `json:"value"`
		Expiration int    `json:"expiration"` // in seconds, 0 for no expiration
	}
	
	if err := c.Bind(&req); err != nil {
		return nil, err
	}
	
	expiration := time.Duration(req.Expiration) * time.Second
	
	result := c.Redis.Set(context.Background(), key, req.Value, expiration)
	if result.Err() != nil {
		return nil, result.Err()
	}
	
	return map[string]interface{}{
		"message": "Key set successfully",
		"key":     key,
		"value":   req.Value,
		"status":  result.Val(),
	}, nil
}

// Get value by key
func getKey(c *gofr.Context) (any, error) {
	key := c.PathParam("key")
	
	result := c.Redis.Get(context.Background(), key)
	if result.Err() != nil {
		return nil, result.Err()
	}
	
	return map[string]interface{}{
		"key":   key,
		"value": result.Val(),
	}, nil
}

// Delete a key
func deleteKey(c *gofr.Context) (any, error) {
	key := c.PathParam("key")
	
	result := c.Redis.Del(context.Background(), key)
	if result.Err() != nil {
		return nil, result.Err()
	}
	
	return map[string]interface{}{
		"message":      "Key deleted",
		"key":          key,
		"deleted_keys": result.Val(),
	}, nil
}

// Set hash fields
func setHash(c *gofr.Context) (any, error) {
	key := c.PathParam("key")
	
	var req map[string]interface{}
	if err := c.Bind(&req); err != nil {
		return nil, err
	}
	
	// Convert map to slice of key-value pairs
	var values []interface{}
	for field, value := range req {
		values = append(values, field, value)
	}
	
	result := c.Redis.HSet(context.Background(), key, values...)
	if result.Err() != nil {
		return nil, result.Err()
	}
	
	return map[string]interface{}{
		"message":    "Hash fields set successfully",
		"key":        key,
		"fields_set": result.Val(),
	}, nil
}

// Get all hash fields
func getHash(c *gofr.Context) (any, error) {
	key := c.PathParam("key")
	
	result := c.Redis.HGetAll(context.Background(), key)
	if result.Err() != nil {
		return nil, result.Err()
	}
	
	return map[string]interface{}{
		"key":    key,
		"fields": result.Val(),
	}, nil
}

// Push to list (left push)
func pushToList(c *gofr.Context) (any, error) {
	key := c.PathParam("key")
	
	var req struct {
		Values []interface{} `json:"values"`
	}
	
	if err := c.Bind(&req); err != nil {
		return nil, err
	}
	
	result := c.Redis.LPush(context.Background(), key, req.Values...)
	if result.Err() != nil {
		return nil, result.Err()
	}
	
	return map[string]interface{}{
		"message":     "Values pushed to list",
		"key":         key,
		"list_length": result.Val(),
	}, nil
}

// Pop from list (right pop)
func popFromList(c *gofr.Context) (any, error) {
	key := c.PathParam("key")
	
	result := c.Redis.RPop(context.Background(), key)
	if result.Err() != nil {
		return nil, result.Err()
	}
	
	return map[string]interface{}{
		"key":   key,
		"value": result.Val(),
	}, nil
}

// Increment counter
func incrementCounter(c *gofr.Context) (any, error) {
	key := c.PathParam("key")
	
	result := c.Redis.Incr(context.Background(), key)
	if result.Err() != nil {
		return nil, result.Err()
	}
	
	return map[string]interface{}{
		"key":   key,
		"value": result.Val(),
	}, nil
}
```

## Common Use Cases

### 1. Caching
Redis is commonly used for caching frequently accessed data:

```go
func getCachedUserData(c *gofr.Context) (any, error) {
	userID := c.PathParam("id")
	cacheKey := "user:" + userID
	
	// Try to get from cache first
	cached := c.Redis.Get(context.Background(), cacheKey)
	if cached.Err() == nil {
		return map[string]interface{}{
			"source": "cache",
			"data":   cached.Val(),
		}, nil
	}
	
	// If not in cache, fetch from database (mock)
	userData := fetchUserFromDB(userID)
	
	// Store in cache for 1 hour
	c.Redis.Set(context.Background(), cacheKey, userData, time.Hour)
	
	return map[string]interface{}{
		"source": "database",
		"data":   userData,
	}, nil
}
```

### 2. Session Management
Using Redis for session storage:

```go
func createSession(c *gofr.Context) (any, error) {
	var req struct {
		UserID string `json:"user_id"`
	}
	
	if err := c.Bind(&req); err != nil {
		return nil, err
	}
	
	sessionID := generateSessionID()
	sessionKey := "session:" + sessionID
	
	sessionData := map[string]interface{}{
		"user_id":    req.UserID,
		"created_at": time.Now().Unix(),
		"ip_address": c.Request.RemoteAddr,
	}
	
	// Store session for 24 hours
	result := c.Redis.HSet(context.Background(), sessionKey, sessionData)
	if result.Err() != nil {
		return nil, result.Err()
	}
	
	c.Redis.Expire(context.Background(), sessionKey, 24*time.Hour)
	
	return map[string]string{
		"session_id": sessionID,
		"message":    "Session created successfully",
	}, nil
}
```

### 3. Rate Limiting
Implementing rate limiting with Redis:

```go
func rateLimitMiddleware(c *gofr.Context) (any, error) {
	clientIP := c.Request.RemoteAddr
	key := "rate_limit:" + clientIP
	
	// Get current count
	current := c.Redis.Get(context.Background(), key)
	
	if current.Err() == nil {
		count, _ := current.Int()
		if count >= 100 { // 100 requests per minute
			return nil, errors.New("rate limit exceeded")
		}
	}
	
	// Increment counter
	c.Redis.Incr(context.Background(), key)
	c.Redis.Expire(context.Background(), key, time.Minute)
	
	return map[string]string{"status": "request allowed"}, nil
}
```

## Data Migrations

GoFr supports data migrations for Redis which allows setting and removing keys, enabling you to manage Redis data changes across different environments.

Create migration files in the `migrations` directory:

```go
// migrations/20231201120000_redis_initial_data.go
package migrations

import "gofr.dev/pkg/gofr"

func up(c *gofr.Context) error {
	// Set initial configuration keys
	c.Redis.Set(context.Background(), "app:version", "1.0.0", 0)
	c.Redis.Set(context.Background(), "app:maintenance", "false", 0)
	return nil
}

func down(c *gofr.Context) error {
	// Remove configuration keys
	c.Redis.Del(context.Background(), "app:version", "app:maintenance")
	return nil
}
```