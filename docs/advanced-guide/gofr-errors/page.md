---
description: "Use GoFr's errors package for structured HTTP and database errors. Predefined types plus custom errors with context simplify error responses and observability."
nextjs:
  metadata:
    title: "Error Handling in GoFr — Structured HTTP and DB Errors"
    description: "Use GoFr's errors package for structured HTTP and database errors. Predefined types plus custom errors with context simplify error responses and observability."
---

# Error Handling

GoFr provides a structured error handling approach to simplify error management in your applications. 
The errors package in GoFr provides functionality for handling errors in GoFr applications. It includes predefined HTTP 
and database errors, as well as the ability to create custom errors with additional context.

## Pre-defined HTTP Errors

GoFr's `http` package offers several predefined error types to represent common HTTP error scenarios. These errors 
automatically handle HTTP status code selection. These include:

{% table %}

- Error Type
- Description
- Status Code

---

- `ErrorInvalidParam`
- Represents an error due to an invalid parameter
- 400 (Bad Request)

---

- `ErrorMissingParam`
- Represents an error due to a missing parameter
- 400 (Bad Request)

---

- `ErrorEntityNotFound`
- Represents an error due to a not found entity
- 404 (Not Found)

---

- `ErrorEntityAlreadyExist`
- Represents an error due to creation of duplicate entity
- 409 (Conflict)

---

- `ErrorInvalidRoute`
- Represents an error for invalid route
- 404 (Not Found)

---

- `ErrorRequestTimeout`
- Represents an error for request which timed out
- 408 (Request Timeout)

---

- `ErrorPanicRecovery`
- Represents an error for request which panicked
- 500 (Internal Server Error)

{% /table %}

#### Usage:
To use the predefined HTTP errors, users need to import the GoFr http package and can simply call them:
```go
import "gofr.dev/pkg/gofr/http"

err := http.ErrorMissingParam{Params: []string{"id"}}
```

## Database Errors
Database errors in GoFr, represented in the `datasource` package, encapsulate errors related to database operations such
as database connection, query failure, availability etc. The `ErrorDB` struct can be used to populate `error` as well as 
any custom message to it. **Status Code: 500 (Internal Server Error)**

#### Usage:
```go
import "gofr.dev/pkg/gofr/datasource"

// Creating a custom error wrapped in  underlying error for database operations
dbErr := datasource.ErrorDB{Err: err, Message: "error from sql db"}

// Adding stack trace to the error
dbErr = dbErr.WithStack()

// Creating a custom error only with error message and no underlying error.
dbErr2 := datasource.ErrorDB{Message : "database connection timed out!"}
```

## Custom Errors
GoFr's error structs implements an interface with `Error() string` and `StatusCode() int` methods, users can override the 
status code by implementing it for their custom error.

Users  can optionally define a log level for your error with the `LogLevel() logging.Level` methods

#### Usage:
```go
type customError struct {
    error string
}

func (c customError) Error() string {
    return fmt.Sprintf("custom error: %s", c.error)
}

func (c customError) StatusCode() int {
    return http.StatusMethodNotAllowed
}

func (c customError) LogLevel() logging.Level {
    return logging.WARN
}
```

## Extended Error Responses

For [RFC 9457](https://www.rfc-editor.org/rfc/rfc9457.html) style error responses with additional fields, implement the ResponseMarshaller interface:

```go
type ResponseMarshaller interface {
    Response() map[string]any
}
```

#### Usage:
```go
type ValidationError struct {
    Field   string
    Message string
    Code    int
}

func (e ValidationError) Error() string    { return e.Message }
func (e ValidationError) StatusCode() int  { return e.Code }

func (e ValidationError) Response() map[string]any {
    return map[string]any{
        "field":   e.Field,
        "type":    "validation_error",
        "details": "Invalid input format",
    }
}
```

> [!NOTE]
> The `message` field is automatically populated from the `Error()` method. Custom fields with the name "message" in the `Response()` map should not be used as they will be ignored in favor of the `Error()` value.

## Hiding Internal Error Details

By default, the `message` in an error response is the text of `err.Error()`. For an error GoFr cannot classify, or one
that wraps a database or driver error, that text can include internal details such as SQL fragments, hostnames or
driver messages. Set `HTTP_HIDE_INTERNAL_ERRORS=true` to keep that text out of client responses.

When it is enabled, the `message` is replaced with `Internal Server Error` when either:

- the error does not implement `StatusCode() int` (an unclassified error, such as one from `errors.New`, or a GoFr
  error wrapped with `fmt.Errorf("...: %w", err)`), or
- the error's `StatusCode()` is 500, such as `datasource.ErrorDB`, `ErrorPanicRecovery`, or a custom error that
  declares 500.

Errors with any other status code keep their message, because that text was written for clients: for example,
`ErrorEntityNotFound` (404), `ErrorInvalidParam` (400) or `ErrorServiceUnavailable` (503).

#### Example:
```go
app.GET("/users/{id}", func(ctx *gofr.Context) (any, error) {
    return nil, errors.New("dial tcp 10.0.3.7:5432: connect: connection refused")
})
```

Without `HTTP_HIDE_INTERNAL_ERRORS` (or with `false`), the response is:
```json
{
  "error": {
    "message": "dial tcp 10.0.3.7:5432: connect: connection refused"
  }
}
```

With `HTTP_HIDE_INTERNAL_ERRORS=true`, the response is:
```json
{
  "error": {
    "message": "Internal Server Error"
  }
}
```

> [!NOTE]
> - The status code never changes, only the message. A 500 stays 500, and a handler that returns data together with
>   an unclassified error still responds with 206 (Partial Content) and its data.
> - Custom fields from a `ResponseMarshaller` error are still included; only `message` is replaced.
> - The original error is still logged on the server with the request's trace ID, so the real cause can be found for
>   the request the client reports.
> - Any value other than `true` or `false` logs a warning at startup and keeps the default behavior.