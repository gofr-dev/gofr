# Connecting to Redis

GoFr simplifies the process of connecting to Redis.

## Setup:

Ensure we have Redis installed on our system.

Optionally, We can use Docker to set up a development environment as described below.

```bash
docker run --name gofr-redis -p 6379:6379 -d redis
```

We can also set up a development environment with password authentication as described below.

```bash
docker run --name gofr-redis -p 2002:6379 -d \
	-e REDIS_PASSWORD=password \
	redis:7.0.5 --requirepass password
```

We can set a sample key `greeting` using the following command:

```bash
docker exec -it gofr-redis bash -c 'redis-cli SET greeting "Hello from Redis."'
```

## Configuration & Usage

GoFr applications relies on environment variables to configure and connect to a Redis server. 
These variables are stored in a file named `.env` located within the configs directory in your project root.

Following configuration keys are required for Redis connectivity:

* `REDIS_HOST`: It specifies the hostname or IP address of your Redis server.
* `REDIS_PORT`: It specifies the port number on which your Redis server is listening. The default Redis port is 6379.
* `REDIS_USER` : This is the user we'll use to connect to your Redis server. We can configure multiple users with different permissions in a single Redis container. For more details, refer to the [official docs](https://redis.io/docs/latest/operate/oss_and_stack/management/security/acl/)
* `REDIS_PASSWORD`: The password is required only if your Redis server is configured for authentication; if authentication is not enabled, no password is necessary.
* `REDIS_DB`: The database number to use for the Redis server. The default value is 0.
```dotenv
APP_NAME=test-service
HTTP_PORT=9000

REDIS_HOST=localhost
REDIS_PORT=6379
REDIS_PASSWORD=password
REDIS_DB=2
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
