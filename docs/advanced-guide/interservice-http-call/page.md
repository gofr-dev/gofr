# Interservice HTTP Calls
GoFr supports inter-service http calls which provide the following benefits :

1. Access to the following method from container - GET, PUT, POST, PATCH, DELETE.
2. Logs and traces for the request.
3. Circuit breaking for enhanced resilience and fault tolerance.
4. Custom Health Check Endpoints

## Usage

### Registering HTTP Service

```go
package main

import (
	"io"
	"time"

	"gofr.dev/pkg/gofr"
	"gofr.dev/pkg/gofr/service"
)

func main() {
	// Create a new application
	a := gofr.New()

	a.AddHTTPService("order", "http://localhost:9000",nil)

	// service with circuit breaker
	a.AddHTTPService("catalogue", "http://localhost:8000",&service.CircuitBreakerConfig{
		Threshold: 4, // after how many failed request circuit breaker will start blocking the requests
		Interval:  1 * time.Second, //  time interval duration between hitting the HealthURL
	},)
	
	// gofr by default hits the `/.well-known/alive` endpoint for service health check.
	// it can be over-ridden using the following option
	a.AddHTTPService("payment", "http://localhost:7000",&service.HealthConfig{
		HealthEndpoint: "my-health",
	},)

	a.GET("/customer", Customer)

	// Run the application
	a.Run()
}
}
```

### Accessing HTTP Service in handler

```go
func Customer(ctx *gofr.Context) (interface{}, error) {
    //Get & Call Another service
    resp, err := ctx.GetHTTPService("payment").Get(ctx, "user", nil)
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