# Errors package in GoFr

GoFr provides a structured error handling approach to simplify error management in your applications. 
The errors package in GoFr provides functionality for handling errors in GoFr applications. It includes predefined errors
for common scenarios and utilities for creating custom errors with additional context.

## Pre-defined Errors

GoFr offers several predefined error types to represent common error scenarios. These include:

- `InvalidParamError`: Represents an error due to an invalid parameter.
- `MissingParamError`: Represents an error due to a missing parameter.
- `EntityNotFoundError`: Represents an error due to a not found entity.
- `MethodNotAllowed`: Represents an error due to a method not allowed on a URL.

#### Usage:
```go
 err := gofrError.MissingParamError{Param: []string{"id"}}
```

# Database Errors and Custom Errors:
Database Errors (DBError) in GoFr are the errors that represents an error due to database connection, query failure, 
availability etc. User can use the database errors in following way:
```go
// Creating a custom error for database operations
dbErr := errors.NewDBError(err, "database operation failed")

// Adding stack trace to the error
dbErr = dbErr.WithStack()
```



For situations beyond predefined errors, GoFr allows you to create custom errors using the NewGofrError function. 
This function takes an underlying error (optional) and a custom message for a clear and informative error representation.

#### Usage:
```go
func ValidateDOB(name string, email string) error {
  if name == "" {
    return errors.NewGofrError(nil, "dob should be greater than 2000.")
  }
  // ... other validations
  return nil
}

```

