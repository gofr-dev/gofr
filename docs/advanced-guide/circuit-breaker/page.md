# Circuit Breaker in GoFr

The Circuit Breaker in GoFr is a powerful mechanism designed to enhance reliability and resilience when interacting with external services. It prevents cascading failures, ensuring graceful degradation and efficient recovery during service outages.

## Configuration

The Circuit Breaker is configured with the following parameters:

- **Enabled**: A flag to enable or disable the Circuit Breaker.
- **Threshold**: Maximum number of consecutive failures before transitioning to the open state.
- **Timeout**: Duration for which the Circuit Breaker stays open before recovery attempts.
- **Interval**: Time interval between health checks in the open state.
- **HealthURL**: URL for health checks to determine the operational status of the underlying service.

## Example Usage

```go
package main

import (
    "time"
    "gofr.dev/pkg/gofr/service"
    "gofr.dev/pkg/gofr"
)

func main() {
    // Create a new application
    a := gofr.New()

    // Add an HTTP service with Circuit Breaker
    a.AddHTTPService("anotherService", "http://localhost:9000", &service.CircuitBreakerConfig{
        Enabled:   true,
        Threshold: 4,
        Timeout:   5 * time.Second,
        Interval:  1 * time.Second,
        HealthURL: "http://localhost:9000/.well-known/health",
    })

    // Add routes and handlers
    a.GET("/example", ExampleHandler)

    // Run the application
    a.Run()
}

func ExampleHandler(c *gofr.Context) (interface{}, error) {
    // Perform your work here

    // Use Circuit Breaker protected HTTP service
    resp, err := c.GetHTTPService("anotherService").Get(c, "/path", nil)
    if err != nil {
        return nil, err
    }

    return resp, nil
}
```

In the example, the Circuit Breaker is applied to an HTTP service, providing robustness in the face of failures. The ExampleHandler demonstrates how to use the Circuit Breaker-protected HTTP service in your application.

This integration ensures that your application can handle failures gracefully, preventing potential service outages and promoting stability.
