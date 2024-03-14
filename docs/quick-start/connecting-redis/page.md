# Connecting to Redis 

GoFr simplifies the process of connecting to Redis. 

## Setup:
Before using Redis with GoFr, you need to have Redis installed. You can use Docker to set up a Redis container:

```bash
docker run --name gofr-redis -p 6379:6379 -d redis
```

To set a sample key, run the following command:

```bash
docker exec -it gofr-redis bash -c 'redis-cli SET greeting "Hello from Redis."'
```

## Configuration & Usage

GoFr requires certain configurations to connect to Redis. The necessary configurations include 
`REDIS_HOST`and `REDIS_PORT`. Update the `.env` file in the configs directory with the following content:

```dotenv
APP_NAME=test-service
HTTP_PORT=9000

REDIS_HOST=localhost
REDIS_PORT=6379
```

Once the Redis configurations are set, you can use Redis in your GoFr application. 
Below is an example of how to retrieve data from Redis in the `main.go` file:

```golang
package main

import (
	"errors"

	"github.com/redis/go-redis/v9"

	"gofr.dev/pkg/gofr"
)

func main() {
	// Initialize GoFr object
	app := gofr.New()

	app.GET("/redis", func(ctx *gofr.Context) (interface{}, error) {
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

The above code demonstrates how to perform Redis operations using the latest GoFr syntax.
You can adapt this example to fit your application's specific requirements.