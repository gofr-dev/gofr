# Error Handling

GoFr provides a structured error handling approach to simplify error management in your applications. 
The errors package in GoFr provides functionality for handling errors in GoFr applications. It includes predefined HTTP 
and database errors, as well as the ability to create custom errors with additional context.

## Pre-defined HTTP Errors

GoFr’s `http` package offers several predefined error types to represent common HTTP error scenarios. These errors 
automatically handle HTTP status code selection. These include:

- `ErrorInvalidParam`: Represents an error due to an invalid parameter.
- `ErrorMissingParam`: Represents an error due to a missing parameter.
- `ErrorEntityNotFound`: Represents an error due to a not found entity.
- `ErrorEntityAlreadyExist`: Represents an error due to creation of duplicate entity.
- `ErrorInvalidRoute`: Represents an error for invalid route.
- `ErrorRequestTimeout`: Represents an error for request which timed out.
- `ErrorPanicRecovery`: Represents an error for request which panicked.

#### Usage:
To use the predefined HTTP errors, users can simply call them using GoFr's http package:
```go
err := http.ErrorMissingParam{Param: []string{"id"}}
```

## Database Errors
Database errors in GoFr, represented in the `datasource` package, encapsulate errors related to database operations such
as database connection, query failure, availability etc. The `ErrorDB` struct can be used to populate `error` as well as 
any custom message to it.

#### Usage:
```go
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