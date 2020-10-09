package main

import (
	"errors"
	"fmt"
	"time"

	"github.com/vikash/gofr/pkg/gofr"
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
	return c.Redis.Get(c, "test").Result()
}

func TraceHandler(c *gofr.Context) (interface{}, error) {
	defer c.Trace("traceHandler").End()

	span2 := c.Trace("handler-work")
	<-time.After(time.Millisecond * 1) // Waiting for 1ms to simulate workload
	span2.End()

	return "Tracing Success", nil
}

func MysqlHandler(c *gofr.Context) (interface{}, error) {
	var value int
	err := c.DB.QueryRowContext(c, "select 2+2").Scan(&value)

	return value, err
}
