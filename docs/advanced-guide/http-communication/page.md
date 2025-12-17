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
func Customer(ctx *gofr.Context) (any, error) {
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

### Additional Configurational Options

GoFr provides its user with additional configurational options while registering HTTP service for communication. These are:

- **ConnectionPoolConfig** - This option allows the user to configure HTTP connection pool settings to optimize performance for high-frequency requests. The default Go HTTP client has `MaxIdleConnsPerHost: 2`, which is often insufficient for microservices making frequent requests to the same host. This configuration allows customizing:
  - `MaxIdleConns`: Maximum idle connections across all hosts (default: 100)
  - `MaxIdleConnsPerHost`: Maximum idle connections per host (critical for performance, default Go value: 2)
  - `IdleConnTimeout`: How long to keep idle connections alive (default: 90 seconds)
  
  **Important**: `ConnectionPoolConfig` must be applied **first** when using multiple options, as it needs access to the underlying HTTP client transport.

- **APIKeyConfig** - This option allows the user to set the `API-Key` Based authentication as the default auth for downstream HTTP Service.
- **BasicAuthConfig** - This option allows the user to set basic auth (username and password) as the default auth for downstream HTTP Service.
- **OAuthConfig** - This option allows user to add `OAuth` as default auth for downstream HTTP Service.
- **CircuitBreakerConfig** - This option allows the user to configure the GoFr Circuit Breaker's `threshold` and `interval` for the failing downstream HTTP Service calls. If the failing calls exceeds the threshold the circuit breaker will automatically be enabled.
- **DefaultHeaders** - This option allows user to set some default headers that will be propagated to the downstream HTTP Service every time it is being called.
- **HealthConfig** - This option allows user to add the `HealthEndpoint` along with `Timeout` to enable and perform the timely health checks for downstream HTTP Service.
- **RetryConfig** - This option allows user to add the maximum number of retry count if before returning error if any downstream HTTP Service fails.

#### Usage:

```go
a.AddHTTPService("cat-facts", "https://catfact.ninja",
	// ConnectionPoolConfig must be applied FIRST
	&service.ConnectionPoolConfig{
		MaxIdleConns:        100,              // Maximum idle connections across all hosts
		MaxIdleConnsPerHost: 20,               // Maximum idle connections per host (increased from default 2)
		IdleConnTimeout:     90 * time.Second, // Keep connections alive for 90 seconds
	},
	
	// Other options can follow in any order
	service.NewAPIKeyConfig("some-random-key"),
	service.NewBasicAuthConfig("username", "password"),
	
    &service.CircuitBreakerConfig{
       Threshold: 4,
       Interval:  1 * time.Second,
  },

   &service.DefaultHeaders{Headers: map[string]string{"key": "value"}},

   &service.HealthConfig{
       HealthEndpoint: "breeds",
  },
   service.NewOAuthConfig("clientID", "clientSecret",
	   "https://tokenurl.com", nil, nil, 0),

  &service.RetryConfig{
      MaxRetries: 5
  },
)
```