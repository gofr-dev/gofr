# Circuit Breaker in HTTP Communication

Calls to remote resources and services can fail due to temporary issues like slow network connections or timeouts, as well as longer-lasting problems such as service unavailability. While transient faults can be mitigated using the "Retry pattern," there are cases where continual retries are futile, such as during severe service failures.

In such scenarios, it's crucial for applications to recognize when an operation is unlikely to succeed and handle the failure appropriately rather than persistently retrying. Indiscriminate use of HTTP retries can even lead to unintentional denial-of-service attacks within the software itself, as multiple clients may flood a failing service with retry attempts.

To prevent this, a defense mechanism like the circuit breaker pattern is essential. Unlike the "Retry pattern" which aims to eventually succeed, the circuit breaker pattern focuses on preventing futile operations. While these patterns can be used together, it's vital for the retry logic to be aware of the circuit breaker's feedback and cease retries if the circuit breaker indicates a non-transient fault.

GoFr inherently provides the functionality, it can be enabled by passing circuit breaker configs as options to `AddHTTPService()` method.

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
			Interval:  1 * time.Second,
		},
	)

	app.GET("/order", Get)

	// Run the application
	app.Run()
}
```

Circuit breaker state changes to open when number of consecutive failed requests increases the threshold.
When it is in open state, GoFr makes request to the aliveness endpoint (default being -  /.well-known/alive) at an equal interval of time provided in config.

To override the default aliveness endpoint {% new-tab-link title="refer" href="/docs/advanced-guide/monitoring-service-health" /%}.
