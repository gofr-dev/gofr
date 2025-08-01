# Error Handling
`serrors` introduces a centralized error handling utility package within the gofr framework.
This establishes a standardized approach to error definition, propagation, and handling across
the system, with the goal of enabling structured logging and consistent error categorization It
also provides a common language and model for errors. Additionally, it enables flexibility in defining
and configuring application-specific errors. Standardize severity levels to make errors more predictable
and actionable. Enable structured logging and instrumentation by making errors machine-readable and codified.
Separate internal errors from user-facing messages to avoid data leakage or confusion.

## Features
- Provide a common language and model for errors.
- Enable flexibility in defining and configuring application-specific errors.
- Standardize severity levels to make errors more predictable and actionable.
- Enable structured logging and instrumentation by making errors machine-readable and codified.
- Separate internal errors from user-facing messages to avoid data leakage or confusion.

## Usage:

### Without Registry
```go
package examples

import (
	"fmt"
	"github.com/pkg/errors"
	"gofr.dev/pkg/gofr/serrors"
)

func main() {
	err := errors.New("db connection error")
	serror := serrors.New(err, err.Error())
	fmt.Println(serrors.GetInternalError(serror, true))
}
```
### With Registry
```go
package examples

import (
	"fmt"
	"gofr.dev/pkg/gofr/serrors"
	"net/http"
)

const EntityNotFound = "EntityNotFound"

var registry = map[string]serrors.Registry{
	EntityNotFound: {
		InternalMessage: "No entity found",
		ExternalStatus:  http.StatusNotFound,
		ExternalMessage: "Requested resource was not found",
		Level:           serrors.INFO,
		SubStatusCode:   "RESOURCE_MISSING",
	},
	"db_conn_failed": {
		SubStatusCode:   "5001",
		InternalMessage: "Database connection failed",
		Retryable:       true,
	},
	"invalid_token": {
		SubStatusCode:   "4011",
		InternalMessage: "Authentication token invalid",
		Retryable:       false,
	},
}

func CreateRegistry() {
	for name, _ := range registry {
		err := serrors.NewFromRegistry(nil, name, registry)
		fmt.Println(serrors.GetInternalError(err, true))
		fmt.Println(serrors.GetExternalError(err))
	}
}
```