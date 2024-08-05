# Testing REST APIs in Go with GoFr

Testing REST APIs ensures that your endpoints function correctly under various conditions. This guide demonstrates how to write tests for GoFr-based REST APIs.

## Mocking Databases in GoFr

Mocking databases allows for isolated testing by simulating various scenarios. GoFr's built-in mock container supports not only SQL databases but also extends to other data stores, including Redis, Cassandra, Key-Value stores, MongoDB, and ClickHouse.

## Example of Unit Testing a REST API Using GoFr

Below is an example of how to test the `Add` method of a handler that interacts with a SQL database.

`main.go`

Here’s an `Add` function for adding a book to the database using GoFr:

```go
package main

import (
	"gofr.dev/pkg/gofr"
	"gofr.dev/pkg/gofr/http"
)

type Book struct {
	Id    int    `json:"id"`
	ISBN  int    `json:"isbn"`
	Title string `json:"title"`
}

func Add(ctx *gofr.Context) (interface{}, error) {
	var book Book

	if err := ctx.Bind(&book); err != nil {
		ctx.Logger.Errorf("error in binding: %v", err)
		return nil, http.ErrorInvalidParam{Params: []string{"body"}}
	}
	
	// we assume the id in the database is set to auto-increment.
	res, err := ctx.SQL.ExecContext(ctx, `INSERT INTO books (title,isbn) VALUES (?,?)`, book.Title, book.ISBN)
	if err != nil {
		return nil, err
	}

	id, err := res.LastInsertId()
	if err != nil {
		return nil, err
	}

	return id, nil
}

func main() {
	// initialise gofr object
	app := gofr.New()
	
	app.POST("/book", Add)

	// Run the application
	app.Run()
}
```

`main_test.go`

Here’s how to write tests using GoFr:
```go
package main

import (
	"bytes"
	"context"
	"database/sql"
	"net/http"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/assert"
	"gofr.dev/pkg/gofr"
	"gofr.dev/pkg/gofr/container"
)

func TestAdd(t *testing.T) {
	type res struct {
		ISBN interface{}
		err   error
	}

	// mock-container
	c, mock := container.NewMockContainer(t)
	
	ctx := &gofr.Context{
		Context:   context.Background(),
		Request:   nil,
		Container: c,
	}

	tests := []struct {
		name         string
		isbn         int
		mockExpect   func()
		expectedRes  interface{}
	}{
		{
			name:  "Successful Add",
			isbn:  12345,
			mockExpect: func() {
				mock.SQL.
					EXPECT().
					ExecContext(ctx, `INSERT INTO books (title,isbn) VALUES (?,?)`, 12345).
					Return(sqlmock.NewResult(12, 1), nil)
			},
			expectedRes: res{
				int64(12), 
				nil,
			},
		},
		{
			name:  "Error on Add",
			isbn:  12345,
			mockExpect: func() {
				mock.SQL.
					EXPECT().
					ExecContext(ctx, `INSERT INTO books (title,isbn) VALUES (?,?)`, 12345).
					Return(nil, sql.ErrConnDone)
			},
			expectedRes: res{
				nil,
				sql.ErrConnDone},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.mockExpect()

			req, _ := http.NewRequest(
				http.MethodPost,
				"/book",
				bytes.NewBuffer([]byte(`{"title":"Book Title","isbn":12345}`)),
			)
			
			req.Header.Set("Content-Type", "application/json")

			request := gofrHttp.NewRequest(req)
			
			ctx.Request = request

			val, err := Add(ctx)
			
			res := res{val, err}
			
			assert.Equal(t, tt.expectedRes, res)
		})
	}
}
```
### Summary

- **Mocking Database Interactions**: Use GoFr's mock container to simulate database interactions.
- **Define Test Cases**: Create table-driven tests to handle various scenarios.
- **Run and Validate**: Ensure that your tests check for expected results and handle errors correctly.

This approach guarantees that your database interactions are tested independently, allowing you to simulate different responses and errors hassle-free.
