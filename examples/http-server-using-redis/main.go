package main

import (
	"errors"
	"time"

	"github.com/bsm/redislock"
	"github.com/redis/go-redis/v9"

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
	app.GET("/redis-lock", RedisLockHandler)

	// Run the application
	app.Run()
}

// RedisSetHandler sets a key-value pair in Redis using the Set Command.
func RedisSetHandler(c *gofr.Context) (interface{}, error) {
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
func RedisGetHandler(c *gofr.Context) (interface{}, error) {
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
func RedisPipelineHandler(c *gofr.Context) (interface{}, error) {
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

// RedisLockHandler demonstrates how to acquire a Redis lock, perform a simple increment operation under
// the lock, and ensure the lock is released afterward.
func RedisLockHandler(c *gofr.Context) (interface{}, error) {
	// Try to obtain the lock
	lock, err := c.Redis.Locker().Obtain(c, "my-lock-key", 5*time.Second, nil)
	if err != nil || errors.Is(err, redislock.ErrNotObtained) {
		return nil, err
	}

	// If lock is successfully acquired, ensure it is released when the handler exits
	defer lock.Release(c)

	// Perform a meaningful operation under the lock
	countKey := "locked-operation-count"
	count, err := c.Redis.Get(c, countKey).Int()
	if err != nil && !errors.Is(err, redis.Nil) {
		return nil, err
	}

	// Increment the count under the lock
	err = c.Redis.Set(c, countKey, count+1, redisExpiryTime*time.Minute).Err()
	if err != nil {
		return nil, err
	}

	return "Lock acquired and counter incremented", nil
}
