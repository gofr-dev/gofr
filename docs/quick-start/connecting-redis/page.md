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
{% /table %}

> For Redis Pub/Sub configuration, see [https://gofr.dev/docs/advanced-guide/using-publisher-subscriber](https://gofr.dev/docs/advanced-guide/using-publisher-subscriber) (Redis Pub/Sub section).

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
{% /table %}

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
