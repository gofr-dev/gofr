# Error Handling

GoFr provides a structured error handling approach to simplify error management in your applications. 
The errors package in GoFr provides functionality for handling errors in GoFr applications. It includes predefined HTTP 
and database errors, as well as the ability to create custom errors with additional context.

## Pre-defined HTTP Errors

GoFr’s `http` package offers several predefined error types to represent common HTTP error scenarios. GThese errors 
automatically handle HTTP status code selection.. These include:

- `ErrorInvalidParam`: Represents an error due to an invalid parameter.
- `ErrorMissingParam`: Represents an error due to a missing parameter.
- `ErrorEntityNotFound`: Represents an error due to a not found entity.

#### Usage:
To use the predefined http errors,users can simply call them using gofr's http package:
```go
 err := http.ErrorMissingParam{Param: []string{"id"}}
```

## Database Errors:
Database errors in GoFr, represented in the `datasource` package, encapsulate errors related to database operations such
as database connection, query failure, availability etc. User can use the `ErrorDB` struct to populate `error` as well as 
any custom message to it:

```go
// Creating a custom error wrapped in  underlying error for database operations
dbErr := datasource.ErrorDB{Err: err, Message: "error from sql db"}

// Adding stack trace to the error
dbErr = dbErr.WithStack()

// Creating a custom error only with error message and no underlying error.
dbErr2 := datasource.ErrorDB{Message : "database connection timed out!"}
```

## Custom Errors

Beyond predefined errors, GoFr allows the creation of custom errors using the `ErrGoFr` struct in 
the `error` package. 

#### Usage:
```go
func ValidateDOB(name string, email string) error {
  if name == "" {
    return errors.ErrGoFr{Message: "dob should be greater than 2000."}
  }
  // ... other validations
  return nil
}

```

> NOTE: Since `GoFrErr` is now an interface with `Error() string` and `StatusCode() int` methods, users can override the 
> status code by implementing it for their custom error.

