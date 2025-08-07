package main

import (
	"time"

	"gofr.dev/pkg/gofr"
)

const redisExpiryTime = 5

func main() {
	// Create a new application
	app := gofr.New()

	// Add routes for Redis operations
	app.GET("/redis/{key}", RedisGetHandler)
	app.POST("/redis", RedisSetHandler)
	app.GET("/redis-pipeline", RedisPipelineHandler)

	// Register an OnStart hook to warm up a cache.
	// This runs before route registration as intended.
	app.OnStart(func(ctx *gofr.Context) error {
		ctx.Logger.Info("Warming up the cache...")

		// Example: Fetch some data and store it in Redis.
		// In a real app, this might come from a database or another service.
		cacheKey := "initial-data"
		cacheValue := "This is some data cached at startup."

		err := ctx.Redis.Set(ctx, cacheKey, cacheValue, 0).Err()
		if err != nil {
			ctx.Logger.Errorf("Failed to warm up cache: %v", err)
			return err // Return the error to halt startup if caching fails.
		}

		ctx.Logger.Info("Cache warmed up successfully!")

		return nil
	})

	// Run the application
	app.Run()
}

// RedisSetHandler sets a key-value pair in Redis using the Set Command.
func RedisSetHandler(c *gofr.Context) (any, error) {
	input := make(map[string]string)

	if err := c.Request.Bind(&input); err != nil {
		return nil, err
	}

	for key, value := range input {
		err := c.Redis.Set(c, key, value, redisExpiryTime*time.Minute).Err()
		if err != nil {
			return nil, err
		}
	}

	return "Successful", nil
}

// RedisGetHandler gets the value from Redis.
func RedisGetHandler(c *gofr.Context) (any, error) {
	key := c.PathParam("key")

	value, err := c.Redis.Get(c, key).Result()
	if err != nil {
		return nil, err
	}

	resp := make(map[string]string)
	resp[key] = value

	return resp, nil
}

// RedisPipelineHandler demonstrates using multiple Redis commands efficiently within a pipeline.
func RedisPipelineHandler(c *gofr.Context) (any, error) {
	pipe := c.Redis.Pipeline()

	// Add multiple commands to the pipeline
	pipe.Set(c, "testKey1", "testValue1", redisExpiryTime*time.Minute)
	pipe.Get(c, "testKey1")

	// Execute the pipeline and get results
	cmds, err := pipe.Exec(c)
	if err != nil {
		return nil, err
	}

	// Process or return the results of each command in the pipeline (implementation omitted for brevity)
	return cmds, nil
}
