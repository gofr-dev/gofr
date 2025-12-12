# Testing REST APIs with GoFr

Testing REST APIs ensures that your endpoints function correctly under various conditions. This guide demonstrates how to write tests for GoFr-based REST APIs.

## Mocking Databases in GoFr

Mocking databases allows for isolated testing by simulating various scenarios. GoFr's built-in mock container supports, not only SQL databases, but also extends to other data stores, including Redis, Cassandra, Key-Value stores, MongoDB, and ClickHouse.

## Example of Unit Testing a REST API Using GoFr

Below is an example of how to test, say the `Add` method of a handler that interacts with a SQL database.

Here’s an `Add` function for adding a book to the database using GoFr:

```go
// main.go
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

func Add(ctx *gofr.Context) (any, error) {
	var book Book

	if err := ctx.Bind(&book); err != nil {
		ctx.Logger.Errorf("error in binding: %v", err)
		return nil, http.ErrorInvalidParam{Params: []string{"body"}}
	}

	// we assume the `id` column in the database is set to auto-increment.
	res, err := ctx.SQL.ExecContext(ctx, `INSERT INTO books (title, isbn) VALUES (?, ?)`, book.Title, book.ISBN)
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
	// initialize gofr object
	app := gofr.New()

	app.POST("/book", Add)

	// Run the application
	app.Run()
}

```

Here’s how to write tests using GoFr:

```go
// main_test.go
package main

import (
	"bytes"
	"context"
	"database/sql"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/assert"

	"gofr.dev/pkg/gofr"
	"gofr.dev/pkg/gofr/container"
	gofrHttp "gofr.dev/pkg/gofr/http"
)

func TestAdd(t *testing.T) {
	type gofrResponse struct {
		result any
		err    error
	}

	// NewMockContainer provides mock implementations for various databases including:
	// Redis, SQL, ClickHouse, Cassandra, MongoDB, and KVStore.
	// These mock can be used to define database expectations in unit tests,
	// similar to the SQL example demonstrated here.
	mockContainer, mock := container.NewMockContainer(t)

	ctx := &gofr.Context{
		Context:   context.Background(),
		Request:   nil,
		Container: mockContainer,
	}

	tests := []struct {
		name             string
		requestBody      string
		mockExpect       func()
		expectedResponse any
	}{
		{
			name:        "Error while Binding",
			requestBody: `title":"Book Title","isbn":12345}`,
			mockExpect: func() {
			},
			expectedResponse: gofrResponse{
				nil,
				gofrHttp.ErrorInvalidParam{Params: []string{"body"}}},
		},
		{
			name:        "Successful Insertion",
			requestBody: `{"title":"Book Title","isbn":12345}`,
			mockExpect: func() {
				mock.SQL.
					ExpectExec(`INSERT INTO books (title, isbn) VALUES (?, ?)`).
					WithArgs("Book Title", 12345).
					WillReturnResult(sqlmock.NewResult(12, 1))
			},
			expectedResponse: gofrResponse{
				int64(12),
				nil,
			},
		},
		{
			name:        "Error on Insertion",
			requestBody: `{"title":"Book Title","isbn":12345}`,
			mockExpect: func() {
				mock.SQL.
					ExpectExec(`INSERT INTO books (title, isbn) VALUES (?, ?)`).
					WithArgs("Book Title", 12345).
					WillReturnError(sql.ErrConnDone)
			},
			expectedResponse: gofrResponse{
				nil,
				sql.ErrConnDone},
		},
		{
			name:        "Error while fetching LastInsertId",
			requestBody: `{"title":"Book Title","isbn":12345}`,
			mockExpect: func() {
				mock.SQL.
					ExpectExec(`INSERT INTO books (title, isbn) VALUES (?, ?)`).
					WithArgs("Book Title", 12345).
					WillReturnError(errors.New("mocked result error"))
			},
			expectedResponse: gofrResponse{
				nil,
				errors.New("mocked result error")},
		},
	}

	for i, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.mockExpect()

			var req *http.Request

			req = httptest.NewRequest(
				http.MethodPost,
				"/book",
				bytes.NewBuffer([]byte(tt.requestBody)),
			)

			req.Header.Set("Content-Type", "application/json")

			request := gofrHttp.NewRequest(req)

			ctx.Request = request

			val, err := Add(ctx)

			response := gofrResponse{val, err}

			assert.Equal(t, tt.expectedResponse, response, "TEST[%d], Failed.\n%s", i, tt.name)
		})
	}
}

```

## Testing HTTP Handlers with Mock Services

When you register multiple services with `WithMockHTTPService`, each service gets its own separate mock instance. This allows you to set different expectations for each service using the `mocks.HTTPServices` map. Use table-driven tests to cover multiple scenarios:

### Important Notes

- **Context Matching**: Always use the exact context from your `gofr.Context` (`ctx.Context`) in expectations. gomock compares contexts by reference, not value, so using `t.Context()` or `context.Background()` will fail.
- **Service Registration**: `WithMockHTTPService("serviceName")` registers the service with the specified name. Each service gets its own separate mock instance.
- **Multiple Services**: Use `mocks.HTTPServices["serviceName"]` to access and set different expectations for each service. Each service has its own mock instance, so expectations are independent.
- **Tests will fail** if the mocked HTTPService is not called as expected or if the context doesn't match.

```go
import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gofr.dev/pkg/gofr"
	"gofr.dev/pkg/gofr/container"
	gofrHttp "gofr.dev/pkg/gofr/http"
)

// Handler that calls HTTP services
func UserProfileHandler(ctx *gofr.Context) (any, error) {
	userID := ctx.PathParam("id")
	service := ctx.GetHTTPService("userService")
	
	resp, err := service.Get(ctx.Context, "/users/"+userID, nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	
	body, _ := io.ReadAll(resp.Body)
	return string(body), nil
}

// Handler that calls multiple HTTP services
func OrderDetailsHandler(ctx *gofr.Context) (any, error) {
	orderID := ctx.PathParam("id")
	
	paymentService := ctx.GetHTTPService("paymentService")
	paymentResp, err := paymentService.Get(ctx.Context, "/payments/"+orderID, nil)
	if err != nil {
		return nil, err
	}
	defer paymentResp.Body.Close()
	
	shippingService := ctx.GetHTTPService("shippingService")
	shippingResp, err := shippingService.Get(ctx.Context, "/shipping/"+orderID, nil)
	if err != nil {
		return nil, err
	}
	defer shippingResp.Body.Close()
	
	return map[string]string{
		"payment_status":  "completed",
		"shipping_status": "in_transit",
	}, nil
}

func TestHTTPHandlers(t *testing.T) {
	tests := []struct {
		name           string
		handler        func(*gofr.Context) (any, error)
		serviceNames   []string
		setupMocks     func(*container.Mocks, *gofr.Context)
		requestPath    string
		wantErr        bool
		wantErrMsg     string
		validateResult func(*testing.T, any)
	}{
		{
			name:         "single service success",
			handler:      UserProfileHandler,
			serviceNames: []string{"userService"},
			setupMocks: func(mocks *container.Mocks, ctx *gofr.Context) {
				mockResp := &http.Response{
					StatusCode: http.StatusOK,
					Body:       io.NopCloser(strings.NewReader(`{"id":"123","name":"John Doe"}`)),
				}
				mocks.HTTPServices["userService"].EXPECT().Get(
					ctx.Context,
					"/users/123",
					nil,
				).Return(mockResp, nil)
			},
			requestPath: "/users/123",
			wantErr:     false,
			validateResult: func(t *testing.T, result any) {
				assert.Contains(t, result.(string), "John Doe")
			},
		},
		{
			name:         "single service error",
			handler:      UserProfileHandler,
			serviceNames: []string{"userService"},
			setupMocks: func(mocks *container.Mocks, ctx *gofr.Context) {
				mocks.HTTPServices["userService"].EXPECT().Get(
					ctx.Context,
					"/users/123",
					nil,
				).Return(nil, errors.New("service unavailable"))
			},
			requestPath: "/users/123",
			wantErr:     true,
			wantErrMsg:  "service unavailable",
		},
		{
			name:         "multiple services success",
			handler:      OrderDetailsHandler,
			serviceNames: []string{"paymentService", "shippingService"},
			setupMocks: func(mocks *container.Mocks, ctx *gofr.Context) {
				paymentResp := &http.Response{
					StatusCode: http.StatusOK,
					Body:       io.NopCloser(strings.NewReader(`{"status":"completed"}`)),
				}
				shippingResp := &http.Response{
					StatusCode: http.StatusOK,
					Body:       io.NopCloser(strings.NewReader(`{"status":"in_transit"}`)),
				}
				mocks.HTTPServices["paymentService"].EXPECT().Get(
					ctx.Context,
					"/payments/456",
					nil,
				).Return(paymentResp, nil)
				mocks.HTTPServices["shippingService"].EXPECT().Get(
					ctx.Context,
					"/shipping/456",
					nil,
				).Return(shippingResp, nil)
			},
			requestPath: "/orders/456",
			wantErr:    false,
			validateResult: func(t *testing.T, result any) {
				resultMap := result.(map[string]string)
				assert.Equal(t, "completed", resultMap["payment_status"])
				assert.Equal(t, "in_transit", resultMap["shipping_status"])
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Register HTTP services
			mockContainer, mocks := container.NewMockContainer(t,
				container.WithMockHTTPService(tt.serviceNames...),
			)

			// Create test request and context
			req := httptest.NewRequest(http.MethodGet, tt.requestPath, nil)
			ctx := &gofr.Context{
				Context:   req.Context(),
				Request:   gofrHttp.NewRequest(req),
				Container: mockContainer,
			}

			// Set up mock expectations
			tt.setupMocks(mocks, ctx)

			// Call the handler
			result, err := tt.handler(ctx)

			if tt.wantErr {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.wantErrMsg)
				assert.Nil(t, result)
			} else {
				require.NoError(t, err)
				if tt.validateResult != nil {
					tt.validateResult(t, result)
				}
			}
		})
	}
}
```

**Key Points**:
- Each service registered via `WithMockHTTPService` gets its own separate mock instance
- Always use `mocks.HTTPServices["serviceName"]` to access and set expectations for a specific service
- Always create the `gofr.Context` with the exact request context (`req.Context()`) that will be used in the handler
- Set expectations on the mock services before calling the handler
- Test both success and error scenarios to ensure your handlers handle all cases correctly

### Summary

- **Mocking Database Interactions**: Use GoFr mock container to simulate database interactions.
- **Mocking HTTP Services**: Use `WithMockHTTPService("serviceName")` to register and mock HTTP services.
- **Context Matching**: Always use `ctx.Context` from your `gofr.Context` in mock expectations, not `t.Context()` or `context.Background()`.
- **Define Test Cases**: Create table-driven tests to handle various scenarios.
- **Run and Validate**: Ensure that your tests check for expected results, and handle errors correctly.

This approach guarantees that your database and HTTP service interactions are tested independently, allowing you to simulate different responses and errors hassle-free.
