# Redis

Redis is an open source, in-memory data structure store, used as a database, cache, and message broker.

## Docker

To run Redis using Docker, use the following command:

```bash
docker run --name gofr-redis -p 6379:6379 -d redis:7-alpine
```

## Connecting to Redis

GoFr provides built-in Redis support. You need to provide the following environment variables:

```bash
REDIS_HOST=localhost
REDIS_PORT=6379
```

### Example

```go
package main

import "gofr.dev/pkg/gofr"

func main() {
    app := gofr.New()

    app.GET("/redis", func(ctx *gofr.Context) (interface{}, error) {
        val := ctx.Redis.Get(ctx, "test-key")
        return val.Val(), val.Err()
    })

    app.Start()
}
```

## Operations

### String Operations

```go
// SET
err := ctx.Redis.Set(ctx, "key", "value", 0).Err()

// GET
val, err := ctx.Redis.Get(ctx, "key").Result()

// INCR
newVal, err := ctx.Redis.Incr(ctx, "counter").Result()
```

### List Operations

```go
// LPUSH
err := ctx.Redis.LPush(ctx, "mylist", "value1").Err()

// RPOP
val, err := ctx.Redis.RPop(ctx, "mylist").Result()

// LRANGE
vals, err := ctx.Redis.LRange(ctx, "mylist", 0, -1).Result()
```

### Hash Operations

```go
// HSET
err := ctx.Redis.HSet(ctx, "myhash", "field1", "value1").Err()

// HGET
val, err := ctx.Redis.HGet(ctx, "myhash", "field1").Result()

// HGETALL
vals, err := ctx.Redis.HGetAll(ctx, "myhash").Result()
```

### Set Operations

```go
// SADD
err := ctx.Redis.SAdd(ctx, "myset", "member1").Err()

// SMEMBERS
members, err := ctx.Redis.SMembers(ctx, "myset").Result()

// SISMEMBER
exists, err := ctx.Redis.SIsMember(ctx, "myset", "member1").Result()
```

## Pub/Sub

Redis supports publish/subscribe messaging:

```go
// Publisher
func publisher(ctx *gofr.Context) (interface{}, error) {
    err := ctx.Redis.Publish(ctx, "mychannel", "message").Err()
    return "Published", err
}

// Subscriber
func subscriber(ctx *gofr.Context) (interface{}, error) {
    pubsub := ctx.Redis.Subscribe(ctx, "mychannel")
    defer pubsub.Close()
    
    msg, err := pubsub.ReceiveMessage(ctx)
    if err != nil {
        return nil, err
    }
    
    return map[string]string{
        "channel": msg.Channel,
        "payload": msg.Payload,
    }, nil
}
```

## Pipeline

For bulk operations, use Redis pipeline:

```go
func pipelineExample(ctx *gofr.Context) (interface{}, error) {
    pipe := ctx.Redis.Pipeline()
    
    incr := pipe.Incr(ctx, "pipeline_counter")
    pipe.Expire(ctx, "pipeline_counter", time.Hour)
    
    _, err := pipe.Exec(ctx)
    if err != nil {
        return nil, err
    }
    
    return incr.Val(), nil
}
```

## Configuration

Additional Redis configuration options:

```bash
REDIS_HOST=localhost
REDIS_PORT=6379
REDIS_PASSWORD=          # Optional
REDIS_DB=0              # Optional
REDIS_MAX_RETRIES=3     # Optional
REDIS_POOL_SIZE=10      # Optional
```