# Redis Integration in GoFr

## Original Guide (Unchanged)

GoFr simplifies the process of connecting to Redis.

### Setup:
Ensure we have Redis installed on our system.

Optionally, we can use Docker to set up a development environment with password authentication as described below.

bash
docker run --name gofr-redis -p 2002:6379 -d \
  -e REDIS_PASSWORD=password \
  redis:7.0.5 --requirepass password


We can set a sample key greeting using the following command:

bash
docker exec -it gofr-redis bash -c "redis-cli SET greeting \"Hello from Redis.\""


### Configuration & Usage:
GoFr applications rely on environment variables to configure and connect to a Redis server.
These variables are stored in a .env file located within the configs directory at your project root.

#### Required Environment Variables:

| Key            | Description                                                              |
|----------------|--------------------------------------------------------------------------|
| REDIS_HOST     | Hostname or IP address of your Redis server                              |
| REDIS_PORT     | Port number your Redis server listens on (default: 6379)                 |
| REDIS_USER     | Redis username; multiple users with ACLs can be configured               |
| REDIS_PASSWORD | Redis password (required only if authentication is enabled)              |
| REDIS_DB       | Redis database number (default: 0)                                       |

---

## New Additions

GoFr provides built-in support for Redis, enabling applications to leverage Redis for fast in-memory data storage, caching, and session management through gofr.Context.

### Redis Interface Definition
go
// Redis provides methods for GoFr applications to communicate with Redis
// through its commands and operations.
type Redis interface {
	HealthChecker

	Get(ctx context.Context, key string) *redis.StringCmd
	Set(ctx context.Context, key string, value interface{}, expiration time.Duration) *redis.StatusCmd
	Del(ctx context.Context, keys ...string) *redis.IntCmd
	Exists(ctx context.Context, keys ...string) *redis.IntCmd
	Expire(ctx context.Context, key string, expiration time.Duration) *redis.BoolCmd

	HGet(ctx context.Context, key, field string) *redis.StringCmd
	HSet(ctx context.Context, key string, values ...interface{}) *redis.IntCmd
	HGetAll(ctx context.Context, key string) *redis.StringStringMapCmd

	LPush(ctx context.Context, key string, values ...interface{}) *redis.IntCmd
	RPop(ctx context.Context, key string) *redis.StringCmd

	SAdd(ctx context.Context, key string, members ...interface{}) *redis.IntCmd
	SMembers(ctx context.Context, key string) *redis.StringSliceCmd

	Incr(ctx context.Context, key string) *redis.IntCmd
	Decr(ctx context.Context, key string) *redis.IntCmd
}


### Configuration
GoFr applications rely on environment variables to configure and connect to a Redis server.
These variables are stored in a .env file located within the configs directory at your project root.

#### Example .env file:

env
REDIS_HOST=localhost
REDIS_PORT=6379
REDIS_USER=default
REDIS_PASSWORD=password


### Setup with Docker

bash
docker run --name gofr-redis -p 2002:6379 -d \
  -e REDIS_PASSWORD=password \
  redis:7.0.5 --requirepass password

docker exec -it gofr-redis bash -c 'redis-cli SET greeting "Hello from Redis."'


### Usage Example
Full GoFr application demonstrating basic Redis operations (key-value, hash, list).

### Common Use Cases
- *Caching*: Store and retrieve user data
- *Session Management*: Create and manage sessions using Redis hashes
- *Rate Limiting*: Track and control request rates using Redis counters

### Data Migrations
GoFr allows data migration files for Redis to set or clean up keys across environments.

go
func up(c *gofr.Context) error {
    c.Redis.Set(context.Background(), "app:version", "1.0.0", 0)
    c.Redis.Set(context.Background(), "app:maintenance", "false", 0)
    return nil
}

func down(c *gofr.Context) error {
    c.Redis.Del(context.Background(), "app:version", "app:maintenance")
    return nil
}
