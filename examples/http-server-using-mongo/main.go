package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"go.mongodb.org/mongo-driver/bson"
	"io"
	"sync"
	"time"

	"github.com/redis/go-redis/v9"

	"gofr.dev/pkg/gofr"
	"gofr.dev/pkg/gofr/datasource"
	"gofr.dev/pkg/gofr/datasource/mongo"
)

func main() {
	// Create a new application
	a := gofr.New()

	//HTTP service with default health check endpoint
	a.AddHTTPService("anotherService", "http://localhost:9000")

	//mongoProvider := mongo.New(mongo.Config{Database: "test", Host: "localhost", Port: 27017, User: "dev", Password: "dev"})
	mongoProvider := mongo.New(mongo.Config{URI: "mongodb://dev:dev@localhost:27017/admin"})
	a.AddMongo(mongoProvider)

	// Add all the routes
	a.GET("/hello", HelloHandler)
	a.GET("/error", ErrorHandler)
	a.GET("/redis", RedisHandler)
	a.GET("/trace", TraceHandler)
	a.GET("/mongo", MongoHandler)

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
		return nil, datasource.ErrorDB{Err: err, Message: "error from redis db"}
	}

	return val, nil
}

func TraceHandler(c *gofr.Context) (interface{}, error) {
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
		Data interface{} `json:"data"`
	}{}

	b, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	_ = json.Unmarshal(b, &data)

	return data.Data, nil
}

func MongoHandler(c *gofr.Context) (interface{}, error) {
	// 检查 MongoDB 连接是否健康
	if err := c.Mongo.Drop(context.Background(), "test"); err != nil {
		return nil, err
	}

	msg, err := c.Mongo.InsertOne(context.Background(), "test2", bson.M{"message": "Hello World!"})
	if err != nil {
		return nil, err
	}
	c.Log(msg)
	return "ok", nil
}
