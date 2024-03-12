package main

import (
	"errors"
	"fmt"
	"reflect"
	"strings"
	"sync"
	"time"

	"github.com/redis/go-redis/v9"
	"gofr.dev/pkg/gofr"
)

func main() {
	// Create a new application
	a := gofr.New()

	type user struct {
		Id         int    `json:"id"`
		Name       string `json:"name"`
		Age        int    `json:"age"`
		IsEmployed bool   `json:"isEmployed"`
	}

	err := CRUDFromStruct(user{}, a)
	if err != nil {
		return
	}

	//HTTP service with default health check endpoint
	a.AddHTTPService("anotherService", "http://localhost:9000")

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

	//Call Another service
	resp, err := c.GetHTTPService("anotherService").Get(c, "redis", nil)
	if err != nil {
		return nil, err
	}

	return resp, nil
}

func MysqlHandler(c *gofr.Context) (interface{}, error) {
	var value int
	err := c.SQL.QueryRowContext(c, "select 2+2").Scan(&value)

	return value, err
}

func CRUDFromStruct(entity interface{}, app *gofr.App) error {
	entityType := reflect.TypeOf(entity)
	if entityType.Kind() != reflect.Struct {
		return errors.New("unexpected field passed for CRUDFromStruct")
	}

	structName := entityType.Name()

	// Assume the first field is the primary key
	primaryKeyField := entityType.Field(0)
	primaryKeyFieldName := strings.ToLower(primaryKeyField.Name)

	// Register GET handler to retrieve all entities
	app.GET(fmt.Sprintf("/%s", structName), func(c *gofr.Context) (interface{}, error) {
		// Implement logic to fetch all entities to fetch all entities from database
		return fmt.Sprintf("GET all %s", structName), nil
	})

	// Register GET handler to retrieve entity by ID
	app.GET(fmt.Sprintf("/%s/{%s}", structName, primaryKeyFieldName), func(c *gofr.Context) (interface{}, error) {
		newEntity := reflect.New(entityType).Interface()
		// Implement logic to fetch entity by ID to fetch entity from database based on ID
		id := c.Request.PathParam("id")
		query := fmt.Sprintf("SELECT * FROM %s WHERE %s = ?", structName, primaryKeyFieldName)

		row := c.SQL.QueryRowContext(c, query, id)

		// Create a slice of pointers to the struct's fields
		dest := make([]interface{}, entityType.NumField())
		val := reflect.ValueOf(newEntity).Elem()
		for i := 0; i < val.NumField(); i++ {
			dest[i] = val.Field(i).Addr().Interface()
		}

		// Scan the result into the struct's fields
		err := row.Scan(dest...)
		if err != nil {
			return nil, err
		}

		c.Logf("GET %s by %s", structName)

		return newEntity, nil
	})

	// Register POST handler to create a new entity
	app.POST(fmt.Sprintf("/%s", structName), func(c *gofr.Context) (interface{}, error) {
		newEntity := reflect.New(entityType).Interface()
		err := c.Bind(newEntity)
		if err != nil {
			return nil, err
		}

		fieldNames := make([]string, 0, entityType.NumField())
		fieldValues := make([]interface{}, 0, entityType.NumField())
		for i := 0; i < entityType.NumField(); i++ {
			field := entityType.Field(i)
			fieldNames = append(fieldNames, field.Name)
			fieldValues = append(fieldValues, reflect.ValueOf(newEntity).Elem().Field(i).Interface())
		}

		stmt := fmt.Sprintf("INSERT INTO %s (%s) VALUES (%s)",
			structName,
			strings.Join(fieldNames, ", "),
			strings.Repeat("?, ", len(fieldNames)-1)+"?",
		)

		_, err = c.SQL.ExecContext(c, stmt, fieldValues...)
		if err != nil {
			return nil, err
		}

		c.Logf("POST %s", structName)

		// Return a success message
		return fmt.Sprintf("%s successfully created with id: %d", structName, fieldValues[0]), nil
	})

	// Register PUT handler to update entity by ID
	app.PUT(fmt.Sprintf("/%s/:%s", structName, primaryKeyFieldName), func(c *gofr.Context) (interface{}, error) {
		// Implement logic to update entity based on request body and ID to parse request body and update entity with ID
		return fmt.Sprintf("PUT %s by %s", structName), nil
	})

	// Register DELETE handler to delete entity by ID
	app.DELETE(fmt.Sprintf("/%s/{%s}", structName, primaryKeyFieldName), func(c *gofr.Context) (interface{}, error) {
		// Implement logic to delete entity by ID
		id := c.PathParam("id")
		query := fmt.Sprintf("DELETE FROM %s WHERE %s = ?", structName, primaryKeyFieldName)

		result, err := c.SQL.ExecContext(c, query, id)
		if err != nil {
			return nil, err
		}

		rowsAffected, err := result.RowsAffected()
		if err != nil {
			return nil, err
		}

		if rowsAffected == 0 {
			return nil, errors.New("entity not found")
		}

		c.Logf("DELETE %s by %s", structName)

		return fmt.Sprintf("%s successfully deleted with id : %v", structName, id), nil
	})

	return nil
}
