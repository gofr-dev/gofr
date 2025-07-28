package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"sync"
	"time"

	"github.com/redis/go-redis/v9"

	"gofr.dev/pkg/gofr"
	"gofr.dev/pkg/gofr/datasource"
)

func main() {
	// Create a new application
	a := gofr.New()

	//HTTP service with default health check endpoint
	a.AddHTTPService("anotherService", "http://localhost:9000")

	// Register an OnStart hook to warm up a cache.
	// This runs before route registration as intended.
	a.OnStart(func(ctx *gofr.Context) error {
		ctx.Container.Logger.Info("Warming up the cache...")

		// Example: Fetch some data and store it in Redis.
		// In a real app, this might come from a database or another service.
		cacheKey := "initial-data"
		cacheValue := "This is some data cached at startup."

		err := ctx.Redis.Set(ctx, cacheKey, cacheValue, 0).Err()
		if err != nil {
			ctx.Container.Logger.Errorf("Failed to warm up cache: %v", err)
			return err // Return the error to halt startup if caching fails.
		}

		ctx.Container.Logger.Info("Cache warmed up successfully!")

		return nil
	})

	// Add all the routes
	a.GET("/hello", HelloHandler)
	a.GET("/error", ErrorHandler)
	a.GET("/redis", RedisHandler)
	a.GET("/trace", TraceHandler)
	a.GET("/mysql", MysqlHandler)

	// Run the application
	a.Run()
}

func HelloHandler(c *gofr.Context) (any, error) {
	name := c.Param("name")
	if name == "" {
		c.Log("Name came empty")
		name = "World"
	}

	return fmt.Sprintf("Hello %s!", name), nil
}

func ErrorHandler(c *gofr.Context) (any, error) {
	return nil, errors.New("some error occurred")
}

func RedisHandler(c *gofr.Context) (any, error) {
	val, err := c.Redis.Get(c, "test").Result()
	if err != nil && err != redis.Nil { // If key is not found, we are not considering this an error and returning "".
		return nil, datasource.ErrorDB{Err: err, Message: "error from redis db"}
	}

	return val, nil
}

func TraceHandler(c *gofr.Context) (any, error) {
	defer c.Trace("traceHandler").End()

	span2 := c.Trace("some-sample-work")
	<-time.After(time.Millisecond * 1) //nolint:wsl    // Waiting for 1ms to simulate workload
	defer span2.End()

	// Ping redis 5 times concurrently and wait.
	count := 5
	wg := sync.WaitGroup{}
	wg.Add(count)

	for i := 0; i < count; i++ {
		go func() {
			c.Redis.Ping(c)
			wg.Done()
		}()
	}
	wg.Wait()

	//Call to Another service
	resp, err := c.GetHTTPService("anotherService").Get(c, "redis", nil)
	if err != nil {
		return nil, err
	}

	defer resp.Body.Close()

	var data = struct {
		Data any `json:"data"`
	}{}

	b, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	_ = json.Unmarshal(b, &data)

	return data.Data, nil
}

func MysqlHandler(c *gofr.Context) (any, error) {
	var value int
	err := c.SQL.QueryRowContext(c, "select 2+2").Scan(&value)
	if err != nil {
		return nil, datasource.ErrorDB{Err: err, Message: "error from sql db"}
	}

	return value, nil
}
