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

- `ErrorMethodNotAllowed`
- Represents an error for a registered route requested with a method it does not accept
- 405 (Method Not Allowed)

---

- `ErrorRequestTimeout`
- Represents an error for request which timed out
- 408 (Request Timeout)

---

- `ErrorPanicRecovery`
- Represents an error for request which panicked
- 500 (Internal Server Error)

{% /table %}

A request for a path that is registered, but not for the method used, is answered with
`ErrorMethodNotAllowed`: a 405 whose `Allow` header lists the methods the path accepts. A path that is
not registered for any method is answered with `ErrorInvalidRoute`.

This includes `HEAD`: a `HEAD` request to a path registered only for `GET` is answered with 405, not
404, and its `Allow` header names `GET`. Register `HEAD` on the path if it should accept it. A health
probe that sends `HEAD` sees this.

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

By default, the message a client receives for an error is the text of `err.Error()`. For an error GoFr cannot
classify, or a database error, that text can include internal details such as SQL fragments, hostnames or driver
messages. Set `HIDE_INTERNAL_ERRORS=true` to keep that text away from clients.

When it is enabled, the message is replaced with `Internal Server Error` when either (gRPC uses its own rule,
described under [Where it applies](#where-it-applies)):

- the error does not implement `StatusCode() int`, such as an error from `errors.New`, a raw driver error, or any
  error wrapped with `fmt.Errorf("...: %w", err)` (the wrapper does not carry the status code of the error inside it), or
- the error is a `datasource.ErrorDB` (or `*datasource.ErrorDB`), which usually carries the driver's own text.

Every other error keeps its message, because an error that declares a status code was written for clients. This
includes GoFr's predefined errors, such as `ErrorEntityNotFound` or `ErrorInvalidParam`, and your own errors with a
`StatusCode()`, **including a custom error that declares 500**.

| Error returned by the handler | Message the client receives |
|---|---|
| `errors.New("pq: relation \"users\" does not exist")` | `Internal Server Error` |
| `datasource.ErrorDB{Err: err, Message: "select failed"}` | `Internal Server Error` |
| `fmt.Errorf("load user: %w", http.ErrorEntityNotFound{Name: "id", Value: "1"})` | `Internal Server Error` |
| a custom error with `StatusCode()` 500 and the text `payment provider is down, retry later` | `payment provider is down, retry later` |
| a custom error with `StatusCode()` 404, whatever its text | its own text |

> [!WARNING]
> A message is kept whenever the error declares a status code, so text you put in your own errors still reaches
> clients. Custom `ResponseMarshaller` fields are also still returned. This option keeps unclassified and database
> error text out of responses; it does not inspect what your own errors say.

#### Example:
```go
app.GET("/users/{id}", func(ctx *gofr.Context) (any, error) {
    return nil, errors.New("dial tcp 10.0.3.7:5432: connect: connection refused")
})
```

Without `HIDE_INTERNAL_ERRORS` (or with `false`), the response is:
```json
{
  "error": {
    "message": "dial tcp 10.0.3.7:5432: connect: connection refused"
  }
}
```

With `HIDE_INTERNAL_ERRORS=true`, the response is:
```json
{
  "error": {
    "message": "Internal Server Error"
  }
}
```

#### Where it applies

- **HTTP responses:** errors returned by handlers and errors written by GoFr's middleware (authentication, rate
  limiting, the `QUERY` content-type check). Only `message` changes: the status code stays the same, custom
  `ResponseMarshaller` fields are still included, and a handler that returns data together with an error still
  responds with 206 (Partial Content) and its data.
- **SSE and NDJSON streams:** the terminal error frame says `{"error":"Internal Server Error"}` when the stream's
  source fails with such an error.
- **GraphQL:** a resolver's error, or a panic in a resolver, appears in `errors[].message` as `Internal Server Error`.
  Errors from parsing and validating the query are not affected.
- **gRPC:** here an error is classified by its gRPC status, not by `StatusCode()`. An error that is not a gRPC
  `status` error is sent as `codes.Internal` with the message `Internal Server Error`, instead of `codes.Unknown`
  with the error's text. This includes your own errors with a `StatusCode()`, such as a custom 500 or
  `ErrorEntityNotFound`: return a `status.Error` with the code and message you want the client to see. A `status`
  error is sent with its own message, including one passed on from a downstream gRPC call; when it is wrapped with
  `%w`, the client sees the wrapper's full text. `context.Canceled` and `context.DeadlineExceeded` keep their code;
  when wrapped, the wrapper's text is dropped. With the option off, the interceptor chain is unchanged.
- **MCP:** a tool's error, and an error from the hook that runs before a tool, reach the AI client as
  `Internal Server Error`.

> [!NOTE]
> - The original error is still logged on the server (for HTTP handlers, with the request's trace ID), so the real
>   cause can be found for the request the client reports. Two exceptions: GoFr does not log a stream source's error,
>   nor the error of a custom `AuthProvider` passed to `middleware.AuthMiddleware` (the built-in authentication
>   errors have a status code and are not hidden), so log those where they are created if you need them.
> - Two GoFr messages that contain no internal detail are not replaced: the stream frame
>   `stream value could not be encoded` and the MCP tool panic text `tool execution failed`.
> - Any value other than `true` or `false` logs a warning at startup and keeps the default behavior.