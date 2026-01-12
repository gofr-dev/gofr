# Circuit Breaker in HTTP Communication

Calls to remote resources and services can fail due to temporary issues like slow network connections or timeouts, service unavailability. While transient faults can be mitigated using the "Retry pattern", there are cases where continual retries are futile, such as during severe service failures.

In such scenarios, it's crucial for applications to recognize when an operation is unlikely to succeed and handle the failure appropriately rather than persistently retrying. Indiscriminate use of HTTP retries can even lead to unintentional denial-of-service attacks within the software itself, as multiple clients may flood a failing service with retry attempts.

To prevent this, a defense mechanism like the circuit breaker pattern is essential. Unlike the "Retry pattern" which aims to eventually succeed, the circuit breaker pattern focuses on preventing futile operations. While these patterns can be used together, it's vital for the retry logic to be aware of the circuit breaker's feedback and cease retries if the circuit breaker indicates a non-transient fault.

GoFr inherently provides the functionality, it can be enabled by passing circuit breaker configs as options to `AddHTTPService()` method.

## How It Works:

The circuit breaker tracks consecutive failed requests for a downstream service.

- **Threshold:** The number of consecutive failed requests after which the circuit breaker transitions to an open state. While open, all requests to that service will fail immediately without making any actual outbound calls, effectively preventing request overflow to an already failing service.



- **Interval:** Once the circuit is open, GoFr starts a background goroutine that periodically checks the health of the service by making requests to its aliveness endpoint (by default: `/.well-known/alive`) at the specified interval. When the service is deemed healthy again, the circuit breaker transitions directly from **Open** to **Closed**, allowing requests to resume.

> GoFr's circuit breaker implementation does not use a **Half-Open** state. Instead, it relies on periodic asynchronous health checks to determine service recovery.

## Failure Conditions

The Circuit Breaker counts a request as "failed" if:
1. An error occurs during the HTTP request (e.g., network timeout, connection refused).
2. The response status code is **greater than 500** (e.g., 502, 503, 504).

> **Note:** HTTP 500 Internal Server Error is **NOT** counted as a failure for the circuit breaker. This distinguishes between application bugs (500) and service availability issues (> 500).

## Health Check Requirement

For the Circuit Breaker to recover from an **Open** state, the downstream service **must** expose a health check endpoint that returns a `200 OK` status code.

- **Default Endpoint:** `/.well-known/alive`
- **Custom Endpoint:** Can be configured using `service.HealthConfig`.

> [!WARNING]
> If the downstream service does not have a valid health check endpoint (returns 404 or other errors), the Circuit Breaker will **never recover** and will remain permanently Open. Ensure your services implement the health endpoint correctly.

## Interaction with Retry

When using both Retry and Circuit Breaker patterns, the **order of wrapping** is critical for effective resilience:

- **Recommended: Retry as the Outer Layer**
  In this configuration, the `Retry` layer wraps the `Circuit Breaker`. Every single retry attempt is tracked by the circuit breaker. If a request retries 5 times, the circuit breaker sees 5 failures. This allows the circuit to trip quickly during a "retry storm," protecting the downstream service from excessive load.

- **Non-Recommended: Circuit Breaker as the Outer Layer**
  If the `Circuit Breaker` wraps the `Retry` layer, it only sees the **final result** of the entire retry loop. Even if a request retries 10 times internally, the circuit breaker only counts it as **1 failure**. This delays the circuit's reaction and can lead to hundreds of futile calls hitting a failing service before the breaker finally trips.

> [!IMPORTANT]
> Always ensure `Retry` is the outer layer by providing the `CircuitBreakerConfig` **before** the `RetryConfig` in the `AddHTTPService` options.

> NOTE: Retries only occur when the target service responds with a status code > 500 (e.g., 502 Bad Gateway, 503 Service Unavailable). 500 Internal Server Error and client errors (4xx) are considered non-transient or bug-related and will not trigger retries.
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
			// Time interval at which circuit breaker will hit the aliveness endpoint.
			Interval: 1 * time.Second,
		},
	)

	app.GET("/order", Get)

	// Run the application
	app.Run()
}
```

Circuit breaker state changes to open when number of consecutive failed requests increases the threshold.
When it is in open state, GoFr makes request to the aliveness endpoint (default being - /.well-known/alive) at an equal interval of time provided in config.

GoFr publishes the following metric to track circuit breaker state:

- `app_http_circuit_breaker_state`: Current state of the circuit breaker (0 for Closed, 1 for Open). This metric is used to visualize a historical timeline of circuit transitions on the dashboard.

> ##### Check out the example of an inter-service HTTP communication along with circuit-breaker in GoFr: [Visit GitHub](https://github.com/gofr-dev/gofr/blob/main/examples/using-http-service/main.go)