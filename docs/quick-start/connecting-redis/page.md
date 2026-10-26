---
description: "Connect a GoFr Go service to Redis via REDIS_HOST and REDIS_PORT. GoFr auto-initializes the client and exposes it on ctx.Redis with built-in observability."
nextjs:
  metadata:
    title: "Connecting Redis in GoFr — Quick-Start Setup"
    description: "Connect a GoFr Go service to Redis via REDIS_HOST and REDIS_PORT. GoFr auto-initializes the client and exposes it on ctx.Redis with built-in observability."
---

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
docker exec gofr-redis redis-cli -a password SET greeting "Hello from Redis."
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

## Example `.env` File

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

## Connecting with a URL (Optional):

Managed Redis providers usually hand out a single connection URL. Set it as `REDIS_URL` instead of the separate variables:

```env
REDIS_URL=rediss://appuser:securepassword@redis.example.com:6380/0
```

- Only the `redis://` and `rediss://` schemes are accepted. `rediss://` turns TLS on and verifies the server against the URL's host name.
- The URL wins. When `REDIS_URL` is set, `REDIS_HOST`, `REDIS_PORT`, `REDIS_USER`, `REDIS_PASSWORD`, `REDIS_DB` and `REDIS_TLS_ENABLED` are ignored, and GoFr logs one warning that names the ignored variables.
- An invalid URL logs an error and Redis is not connected, the same as a missing `REDIS_HOST`. The error never contains the password. A URL with no host (`redis://:6379`) is invalid; GoFr does not fall back to `localhost`.
- A certificate cannot be put in a URL, so `REDIS_TLS_CA_CERT`, `REDIS_TLS_CERT` and `REDIS_TLS_KEY` still apply to a `rediss://` URL.
- The database in the URL (`/0` above) is used by `ctx.Redis`. Redis Pub/Sub keeps using `REDIS_PUBSUB_DB` (default `15`).

## Pool and Timeout Settings (Optional):

These variables tune the connection pool, with or without `REDIS_URL`. Each one overrides the matching URL parameter (`?pool_size=20`), and an invalid or negative value logs a warning and keeps the default.

```env
REDIS_POOL_SIZE=20
REDIS_MIN_IDLE_CONNS=2
REDIS_DIAL_TIMEOUT=2s
REDIS_READ_TIMEOUT=500ms
REDIS_WRITE_TIMEOUT=500ms
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
