# Circuit Breaker in HTTP Communication

Calls to remote resources and services can fail due to temporary issues like slow network connections or timeouts, service unavailability. While transient faults can be mitigated using the "Retry pattern", there are cases where continual retries are futile, such as during severe service failures.

In such scenarios, it's crucial for applications to recognize when an operation is unlikely to succeed and handle the failure appropriately rather than persistently retrying. Indiscriminate use of HTTP retries can even lead to unintentional denial-of-service attacks within the software itself, as multiple clients may flood a failing service with retry attempts.

To prevent this, a defense mechanism like the circuit breaker pattern is essential. Unlike the "Retry pattern" which aims to eventually succeed, the circuit breaker pattern focuses on preventing futile operations. While these patterns can be used together, it's vital for the retry logic to be aware of the circuit breaker's feedback and cease retries if the circuit breaker indicates a non-transient fault.

GoFr inherently provides the functionality, it can be enabled by passing circuit breaker configs as options to `AddHTTPService()` method.

## How It Works:

The circuit breaker tracks consecutive failed requests for a downstream service.

- **Threshold:** The number of consecutive failed requests after which the circuit breaker transitions to an open state. While open, all requests to that service will fail immediately without making any actual outbound calls, effectively preventing request overflow to an already failing service.



- **Interval:** Once the circuit is open, GoFr starts a background goroutine that periodically checks the health of the service by making requests to its aliveness endpoint at the specified interval. When the service is deemed healthy again, the circuit breaker closes, allowing requests to resume.



- **HealthEndpoint:** (Optional) A custom health endpoint to use for circuit breaker recovery checks. By default, GoFr uses `/.well-known/alive`. If the downstream service doesn't expose this endpoint, you can specify a custom endpoint (e.g., `health`, `status`, or any valid endpoint that returns HTTP 200 when the service is healthy).



- **HealthTimeout:** (Optional) The timeout in seconds for health check requests. Defaults to 5 seconds if not specified.



> NOTE: Retries only occur when the target service responds with a 500 Internal Server Error. Errors like 400 Bad Request or 404 Not Found are considered non-transient and will not trigger retries.
## Usage

```go
package main

import (
	"time"

	"gofr.dev/pkg/gofr"
	"gofr.dev/pkg/gofr/service"
)

func main() {
	// Create a new application
	app := gofr.New()

	app.AddHTTPService("order", "https://order-func",
		&service.CircuitBreakerConfig{
			// Number of consecutive failed requests after which circuit breaker will be enabled
			Threshold: 4,
			// Time interval at which circuit breaker will hit the health endpoint.
			Interval: 1 * time.Second,
			// Custom health endpoint for circuit breaker recovery (optional)
			// Use this when the downstream service doesn't expose /.well-known/alive
			HealthEndpoint: "health",
			// Timeout for health check requests in seconds (optional, defaults to 5)
			HealthTimeout: 10,
		},
	)

	app.GET("/order", Get)

	// Run the application
	app.Run()
}
```

Circuit breaker state changes to open when number of consecutive failed requests increases the threshold.
When it is in open state, GoFr makes request to the health endpoint (default being - /.well-known/alive, or the custom endpoint if configured) at an equal interval of time provided in config.

> ##### Check out the example of an inter-service HTTP communication along with circuit-breaker in GoFr: [Visit GitHub](https://github.com/gofr-dev/gofr/blob/main/examples/using-http-service/main.go)