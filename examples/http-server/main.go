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

	// Add all the routes
	a.GET("/hello", HelloHandler)
	a.GET("/error", ErrorHandler)
	a.GET("/redis", RedisHandler)
	a.GET("/trace", TraceHandler)
	a.GET("/mysql", MysqlHandler)

	// QUERY (RFC 10008): a safe, idempotent method that carries the query in the
	// request body. Read the body via ctx.Bind, the same as a POST.
	a.QUERY("/search", SearchHandler)

	// Outbound QUERY: same method available on the HTTP service client. Mirrors
	// GET/POST/etc. — takes a body and returns the raw response for you to decode.
	a.GET("/proxy-search", ProxySearchHandler)

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

// SearchHandler demonstrates the HTTP QUERY method: the search criteria arrive in
// the request body and are echoed back as the query result.
func SearchHandler(c *gofr.Context) (any, error) {
	criteria := struct {
		Filter string `json:"filter"`
	}{}

	if err := c.Bind(&criteria); err != nil {
		return nil, err
	}

	return map[string]string{"matched": criteria.Filter}, nil
}

// ProxySearchHandler demonstrates the outbound HTTP QUERY method: it forwards
// a filter to the downstream `anotherService` at `/search` via svc.Query and
// returns whatever the downstream matched. Mirrors the inbound SearchHandler
// on the other side of the wire.
func ProxySearchHandler(c *gofr.Context) (any, error) {
	body := []byte(`{"filter":"golang"}`)

	resp, err := c.GetHTTPService("anotherService").
		QueryWithHeaders(c, "search", nil, body, map[string]string{"Content-Type": "application/json"})
	if err != nil {
		return nil, err
	}

	defer resp.Body.Close()

	var out struct {
		Data map[string]string `json:"data"`
	}

	b, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	if err := json.Unmarshal(b, &out); err != nil {
		return nil, err
	}

	return out.Data, nil
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
	// Waiting for 1ms to simulate workload
	<-time.After(time.Millisecond * 1) //nolint:wsl
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

	// Call to Another service
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

	if err := json.Unmarshal(b, &data); err != nil {
		return nil, err
	}

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
