# Connecting Redis

GoFr supports connection to redis and automatically manages the connection pool, connection retry etc. and it is done by adding configs.

## Setup

Run the following docker commands to install Redis.

```bash
docker run --name gofr-redis -p 6379:6379 -d redis
```

To set the key _greeting_, run the following command

```bash
docker exec -it gofr-redis bash -c 'redis-cli SET greeting "Hello from Redis."'
```

## Configuration & Usage

Let's use a redis datastore in our Hello API server to return values from redis.
**REDIS_HOST** and **REDIS_PORT** configs should be added to allow GoFr to automatically connect to Redis.

After adding redis configs `.env` will be updated to the following.

```dotenv
# configs/.env
APP_NAME=test-service
HTTP_PORT=9000

REDIS_HOST=localhost
REDIS_PORT=6379
```

After adding code to retrieve data from redis datastore `main.go` will be updated to the following.

```go
package main

import "gofr.dev/pkg/gofr"

func main() {
    // initialise gofr object
    app := gofr.New()

    app.GET("/greet", func(ctx *gofr.Context) (interface{}, error) {
		// Get the value using the redis instance
		value, err := ctx.Redis.Get(ctx.Context, "greeting").Result()

        return value, err
    })

    // Starts the server, it will listen on the default port 8000.
    // it can be over-ridden through configs
    app.Start()
}
```

Call the [/greet](http://localhost:9000/greet) endpoint, and you should get the following output

```json
{
  "data": "Hello from Redis."
}
```

---

GoFr also supports these Redis configs:

| Config Name          | Description                                     |
| -------------------- | ----------------------------------------------- |
| **REDIS_PASSWORD**   | Password for the Redis server.                  |
| **REDIS_SSL**        | Enable SSL/TLS Authentication for Redis.        |
| **REDIS_CONN_RETRY** | Connection retry interval for Redis in seconds. |
