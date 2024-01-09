package main

import (
	"errors"
	"fmt"
	"gofr.dev/pkg/gofr/service"
	"sync"
	"time"

	"github.com/go-redis/redis/v8"

	"gofr.dev/pkg/gofr"
)

func main() {
	// Create a new application
	a := gofr.New()

	// Add all the routes
	a.GET("/hello", HelloHandler)
	a.GET("/error", ErrorHandler)
	a.GET("/redis", RedisHandler)
	a.GET("/trace", TraceHandler)
	a.GET("/mysql", MysqlHandler)

	// Run the application
	a.Run()
}

func HelloHandler(c *gofr.Context) (interface{}, error) {
	name := c.Param("name")
	if name == "" {
		c.Log("Name came empty")
		name = "World"
	}

	return fmt.Sprintf("Hello %s!", name), nil
}

func ErrorHandler(c *gofr.Context) (interface{}, error) {
	return nil, errors.New("some error occurred")
}

func RedisHandler(c *gofr.Context) (interface{}, error) {
	val, err := c.Redis.Get(c, "test").Result()
	if err != nil && err != redis.Nil { // If key is not found, we are not considering this an error and returning "".
		return nil, err
	}

	return val, nil
}

func TraceHandler(c *gofr.Context) (interface{}, error) {
	defer c.Trace("traceHandler").End()

	span2 := c.Trace("some-sample-work")
	<-time.After(time.Millisecond * 1) //nolint:wsl    // Waiting for 1ms to simulate workload
	span2.End()

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

	// Call Another service
	anotherService := service.NewHTTPService("http://localhost:9000")
	anotherService.Get(c, "redis", nil)

	return "Tracing Success", nil
}

func MysqlHandler(c *gofr.Context) (interface{}, error) {
	var value int
	err := c.DB.QueryRowContext(c, "select 2+2").Scan(&value)

	return value, err
}
