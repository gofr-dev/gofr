package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"sync"
	"time"

	"github.com/redis/go-redis/v9"
	"gofr.dev/pkg/gofr/service"

	"gofr.dev/pkg/gofr"
	"gofr.dev/pkg/gofr/service"
)

func main() {
	// Create a new application
	a := gofr.New()

	a.AddHTTPService("anotherService", "http://localhost:9000", &service.CircuitBreakerConfig{
		Threshold: 4,
		Timeout:   5 * time.Second,
		Interval:  1 * time.Second,
		HealthURL: "http://localhost:9000/.well-known/health",
	})

	a.AddHTTPService("cachedService", "http://localhost:9000", &service.Cache{
		TTL: 5 * time.Second,
	})

	// Add all the routes
	a.GET("/hello", HelloHandler)
	a.GET("/error", ErrorHandler)
	a.GET("/redis", RedisHandler)
	a.GET("/trace", TraceHandler)
	a.GET("/mysql", MysqlHandler)
	a.GET("/cachedService", CachedServiceHandler)

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

	//Call Another service
	resp, err := c.GetHTTPService("anotherService").Get(c, "redis", nil)
	if err != nil {
		return nil, err
	}

	return resp, nil
}

func MysqlHandler(c *gofr.Context) (interface{}, error) {
	var value int
	err := c.DB.QueryRowContext(c, "select 2+2").Scan(&value)

	time.Sleep(3 * time.Second)
	return value, err
}

func CachedServiceHandler(c *gofr.Context) (interface{}, error) {
	var res struct {
		Data interface{} `json:"data"`
	}

	resp, err := c.GetHTTPService("cachedService").Get(c, "mysql", nil)
	defer resp.Body.Close()
	bodyBytes, err := io.ReadAll(resp.Body)
	_ = json.Unmarshal(bodyBytes, &res)

	return res.Data, err
}
