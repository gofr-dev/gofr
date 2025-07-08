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

| Key           | Description                                      |
|---------------|--------------------------------------------------|
| `REDIS_HOST`  | Hostname or IP address of your Redis server      |
| `REDIS_PORT`  | Port number your Redis server listens on (default: `6379`) |
| `REDIS_USER`  | Redis username; multiple users with ACLs can be configured. [See official docs](https://redis.io/docs/latest/operate/oss_and_stack/management/security/acl/) |
| `REDIS_PASSWORD` | Redis password (required only if authentication is enabled) |
| `REDIS_DB`    | Redis database number (default: `0`)             |

### Optional TLS Support:

| Key                        | Description                                      |
|----------------------------|--------------------------------------------------|
| `REDIS_TLS_ENABLED`        | Set to `"true"` to enable TLS                    |
| `REDIS_TLS_CA_CERT_PATH`   | File path to the CA certificate used to verify the Redis server |
| `REDIS_TLS_CERT_PATH`      | File path to the client certificate (for mTLS)   |
| `REDIS_TLS_KEY_PATH`       | File path to the client private key (for mTLS)   |

### Example `.env` File

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

## Common Use Case: Caching

Use Redis for caching to improve performance with a cache-aside pattern, such as for product data:

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

    app.GET("/products/{id}", func(ctx *gofr.Context) (interface{}, error) {
        productID := ctx.PathParam("id")
        cacheKey := "product:" + productID

        // Check cache
        cached := ctx.Redis.Get(cacheKey)
        if cached.Err() == nil {
            var product Product
            if err := json.Unmarshal([]byte(cached.Val()), &product); err == nil {
                return product, nil
            }
        }

        // Cache miss: fetch from database (simulated)
        product := Product{
            ID:          1,
            Name:        "Sample Product",
            Description: "A sample product for testing",
            Price:       99.99,
        }

        // Store in cache for 1 hour
        productJSON, _ := json.Marshal(product)
        ctx.Redis.Set(cacheKey, productJSON, time.Hour)

        return product, nil
    })

    app.Run()
}
```

## Best Practices

- **Error Handling**: Check for `redis.Nil` to handle missing keys gracefully.
- **TLS**: Enable `REDIS_TLS_ENABLED` for secure connections in production.
- **Connection Management**: GoFr manages Redis connections automatically; adjust environment variables as needed for performance.
