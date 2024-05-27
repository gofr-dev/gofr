# Inter-Service HTTP Calls

GoFr promotes microservice architecture and to facilitate the same, it provides the support to initialize HTTP services
at application level using `AddHTTPService()` method.

Support for inter-service HTTP calls provide the following benefits:
1. Access to the methods from container - GET, PUT, POST, PATCH, DELETE.
2. Logs and traces for the request.
3. {% new-tab-link newtab=false title="Circuit breaking" href="/docs/advanced-guide/circuit-breaker" /%} for enhanced resilience and fault tolerance.
4. {% new-tab-link newtab=false title="Custom Health Check" href="/docs/advanced-guide/monitoring-service-health" /%} Endpoints

## Usage

### Registering a simple HTTP Service

GoFr allows registering a new HTTP service using the application method `AddHTTPService()`.
It takes in a service name and service address argument to register the dependent service at application level.
Registration of multiple dependent services is quite easier, which is a common use case in a microservice architecture.

> The services instances are maintained by the container.

Other provided options can be added additionally to coat the basic HTTP client with features like circuit-breaker and
custom health check and add to the functionality of the HTTP service.
The design choice for this was made such as many options as required can be added and are order agnostic,
i.e. the order of the options is not important.

> Service names are to be kept unique to one service.

```go
app.AddHTTPService(<service_name> , <service_address>)
```

#### Example
```go
package main

import (
	"gofr.dev/pkg/gofr"
)

func main() {
	// Create a new application
	app := gofr.New()

	// register a payment service which is hosted at http://localhost:9000
	app.AddHTTPService("payment", "http://localhost:9000")

	app.GET("/customer", Customer)

	// Run the application
	app.Run()
}
```

### Accessing HTTP Service in handler

The HTTP service client is accessible anywhere from `gofr.Context` that gets passed on from the handler.
Using the `GetHTTPService` method with the service name that was given at the time of registering the service,
the client can be retrieved as shown below:

```go
svc := ctx.GetHTTPService(<service_name>)
```

```go
func Customer(ctx *gofr.Context) (interface{}, error) {
    // Get the payment service client
    paymentSvc := ctx.GetHTTPService("payment")

	// Use the Get method to call the GET /user endpoint of payments service
	resp, err := paymentSvc.Get(ctx, "user", nil)
    if err != nil {
        return nil, err
    }

	defer resp.Body.Close()

    body, err := io.ReadAll(resp.Body)
    if err != nil {
        return nil, err
    }

    return string(body), nil
}
```
