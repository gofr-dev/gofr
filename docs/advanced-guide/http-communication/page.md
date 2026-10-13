---
description: "Call other services from a GoFr handler with AddHTTPService. The instrumented client adds traces, metrics, retries, and circuit breakers to outbound HTTP calls."
nextjs:
  metadata:
    title: "Inter-Service HTTP in GoFr — AddHTTPService Client"
    description: "Call other services from a GoFr handler with AddHTTPService. The instrumented client adds traces, metrics, retries, and circuit breakers to outbound HTTP calls."
---

# Inter-Service HTTP Calls

GoFr promotes microservice architecture and to facilitate the same, it provides the support to initialize HTTP services
at application level using `AddHTTPService()` method.

Support for inter-service HTTP calls provide the following benefits:
1. Access to the methods from container - GET, PUT, POST, PATCH, DELETE, QUERY.
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
app.AddHTTPService(<service_name>, <service_address>)
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

#### Available Methods
The HTTP service client provides methods for making requests to downstream services:

- `Get(ctx, path, queryParams)`

- `Post(ctx, path, queryParams, body)`

- `Put(ctx, path, queryParams, body)`

- `Patch(ctx, path, queryParams, body)`

- `Delete(ctx, path, body)`

- `Query(ctx, path, queryParams, body)`

**For scenarios requiring custom header propagation (authentication, multi-tenancy, user identity propagation), use the `WithHeaders` variants:**

- `GetWithHeaders(ctx, path, queryParams, headers)`

- `PostWithHeaders(ctx, path, queryParams, body, headers)`

- `PutWithHeaders(ctx, path, queryParams, body, headers)`

- `PatchWithHeaders(ctx, path, queryParams, body, headers)`

- `DeleteWithHeaders(ctx, path, body, headers)`

- `QueryWithHeaders(ctx, path, queryParams, body, headers)`

> **QUERY** (RFC 10008) is a safe, idempotent method that carries the query in the
> request body — bridging the gap between GET (no body) and POST (not safe/idempotent).
> Like GET, QUERY calls are wrapped by the circuit-breaker and retry options when configured.

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

// For microservice patterns involving authentication (ex: JWT Token Forwarding), use WithHeaders methods to forward custom headers.
func GatewayHandler(ctx *gofr.Context) (any, error) {
	authInfo := ctx.GetAuthInfo()
	claims := authInfo.GetClaims()

	userID, _ := claims.GetSubject()

	headers := map[string]string{
		"X-User-ID": userID,
	}

	userSvc := ctx.GetHTTPService("user-service")
	resp, err := userSvc.GetWithHeaders(ctx, "api/user/profile", nil, headers)
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
  - `MaxIdleConns`: Maximum idle connections across all hosts. If not explicitly set (0), a default of 100 will be used.
  - `MaxIdleConnsPerHost`: Maximum idle connections per host (critical for performance). If set to 0, Go's DefaultMaxIdleConnsPerHost (2) will be used. Negative values will cause validation error.
  - `IdleConnTimeout`: How long to keep idle connections alive. If not explicitly set (0), a default of 90 seconds will be used.

  **Important**: `ConnectionPoolConfig` must be applied **first** when using multiple options, as it needs access to the underlying HTTP client transport.

- **APIKeyConfig** - This option allows the user to set the `API-Key` Based authentication as the default auth for downstream HTTP Service.
- **BasicAuthConfig** - This option allows the user to set basic auth (username and password) as the default auth for downstream HTTP Service.

**Important:** The password must be base64 encoded in your configuration/environment variables. GoFr will decode it internally before creating the Authorization header.


**Example:**
```bash
# Generate base64 encoded password
echo -n "your-password" | base64
# Output: eW91ci1wYXNzd29yZA==
```

- **OAuthConfig** - This option allows the user to add `OAuth` as default auth for downstream HTTP Service.
- **FileTokenAuthConfig** - This option reads a bearer token from a file and refreshes it periodically, injecting an `Authorization: Bearer <token>` header on every outgoing call. It is intended for Kubernetes projected service account tokens, which are rotated on disk. See [File-Based Token Authentication](#file-based-token-authentication) below.
- **CircuitBreakerConfig** - This option allows the user to configure the GoFr Circuit Breaker's `threshold` and `interval` for the failing downstream HTTP Service calls. If the failing calls exceeds the threshold the circuit breaker will automatically be enabled.
- **DefaultHeaders** - This option allows the user to set some default headers that will be propagated to the downstream HTTP Service every time it is being called.
- **HealthConfig** - This option allows the user to add the `HealthEndpoint` along with `Timeout` to enable and perform the timely health checks for downstream HTTP Service.
- **RetryConfig** - This option allows the user to add the maximum number of retry count before returning error if any downstream HTTP Service fails. Retries are triggered for network errors and status codes **> 500** (e.g., 503 Service Unavailable). HTTP 500 is not retried. Retries stop as soon as the caller's context is canceled or past its deadline -- including a deadline that passes while an attempt is already on the wire -- and the error of the attempt it ended is returned.
- **RetryPolicy** - A configurable alternative to `RetryConfig`: waits between attempts (backoff with jitter), honors `Retry-After`, retries only methods that are safe to repeat, bounds each attempt, and caps retries with a budget. See [Retry Policy](#retry-policy) below. `RetryConfig` keeps its behavior unchanged.
- **RateLimiterConfig** -  This option allows the user to configure rate limiting for downstream service calls using token bucket algorithm. It controls the request rate to prevent overwhelming dependent services and supports both in-memory and Redis-based implementations.

**Rate Limiter Store: Customization**
GoFr allows you to use a custom rate limiter store by implementing the RateLimiterStore interface. This enables integration with any backend (e.g., Redis, database, or custom logic)

**Interface:**

```go
type RateLimiterStore interface {
	Allow(ctx context.Context, key string, config RateLimiterConfig) (allowed bool, retryAfter time.Duration, err error)
	StartCleanup(ctx context.Context)
	StopCleanup()
}
```

#### Usage:

```go
rc := redis.NewClient(a.Config, a.Logger(), a.Metrics())

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
		MaxRetries: 5,
	},

	&service.RateLimiterConfig{
		Requests: 5,
		Window:   time.Minute,
		Burst:    10,
		Store:    service.NewRedisRateLimiterStore(rc), // Skip this field to use in-memory store
	},
)
```

**Best Practices:**
- For distributed systems: It is strongly recommended to use Redis-based store (`NewRedisRateLimiterStore`) to ensure consistent rate limiting across multiple instances of your application.
- For single-instance applications: The default in-memory store (`NewLocalRateLimiterStore`) is sufficient and provides better performance.
- Rate configuration: Set Burst higher than Requests to allow short traffic bursts while maintaining average rate limits.

### Retry Policy

`RetryConfig` retries immediately, replays every method, and has no limit on load. When an upstream is slow or overloaded, every caller in a fleet then retries at once, which keeps it overloaded. `RetryPolicy` is the option for that case:

```go
a.AddHTTPService("orders", "https://orders.example.com",
	// Each option wraps the ones registered before it. Registering the circuit breaker BEFORE the
	// retry policy puts it inside the retries: every attempt is counted by the breaker, and an open
	// breaker stops the retries.
	&service.CircuitBreakerConfig{Threshold: 4, Interval: time.Second},

	&service.RetryPolicy{
		MaxAttempts:       4, // the first attempt plus up to 3 retries
		Backoff:           &service.ExponentialBackoff{Base: 100 * time.Millisecond, Max: 2 * time.Second},
		RespectRetryAfter: true,
		PerAttemptTimeout: 2 * time.Second,
		Budget:            &service.RetryBudget{Ratio: 0.2, MinPerSecond: 10},
	},
)
```

| Field | Default (zero value) | Meaning |
|---|---|---|
| `MaxAttempts` | `0` or `1`: no retry | Total attempts, including the first. |
| `Backoff` | `nil`: no wait | The wait before retry `n` (`n` = 1 for the first retry). `ConstantBackoff{Delay}` waits the same each time. `ExponentialBackoff{Base, Max, Multiplier}` waits a random time between 0 and `min(Max, Base × Multiplier^(n-1))` ("full jitter"); `Multiplier` defaults to 2. |
| `RetryOn` | `StandardRetryable` | Decides whether a result is retried. `StandardRetryable` retries network errors (including timeouts, such as an attempt cut off by `PerAttemptTimeout`), a `RateLimitError`, and the statuses 408, 429, 502, 503 and 504. It never retries 500, 501 or 505. A custom rule can call `StandardRetryable` and add to it. |
| `RespectRetryAfter` | `false` | Wait for the server's `Retry-After` (seconds or an HTTP date) on a 429 or 503, and for `RetryAfter` of a `RateLimitError` from `RateLimiterConfig`, instead of the backoff. |
| `MaxRetryAfter` | `30s` | The longest `Retry-After` that is honored; a longer one is cut to this. |
| `Methods` | `GET`, `HEAD`, `OPTIONS`, `PUT`, `DELETE`, `QUERY` | Methods that are always retried. A request with any other method is sent once, unless it carries the idempotency key header. |
| `IdempotencyKeyHeader` | `Idempotency-Key` | A request carrying this header with a non-empty value is retried whatever its method (for example a `POST`), since the server can use the key to drop the duplicate. |
| `PerAttemptTimeout` | `0`: none | The longest one attempt may wait for the response headers. An attempt that takes longer is abandoned and retried. Reading the body of a successful response is not limited. An attempt that no retry can follow is not limited either: the last attempt, and every attempt of a request that is never retried (such as a `POST` without an idempotency key). Cutting such an attempt off would report a timeout for a request the server may still complete; only the caller's context bounds it. |
| `Budget` | `nil`: unlimited | Caps retries per service to `Ratio` × first attempts plus `MinPerSecond` per second, counted over `TTL` (default 10s). When the budget is spent, the last result is returned without retrying. A budget with both `Ratio` and `MinPerSecond` left at `0`, such as `&service.RetryBudget{}`, uses `Ratio` 0.2 and `MinPerSecond` 10. To make no retries, set `MaxAttempts` to `1`. |

How a call proceeds:

1. A request is retried only if its method is in `Methods` or it carries the idempotency key header. So by default `POST` and `PATCH` are sent once unless they carry `Idempotency-Key`.
2. Before each retry, the policy waits for `Retry-After` (when `RespectRetryAfter` is set and the server sent one) or for the backoff.
3. If the wait would end after the caller's context deadline (or, when `PerAttemptTimeout` is set, would leave less than one full attempt before it), or the context ends during the wait, the policy stops and returns the last result, with its body still readable. (While the policy waits, a small body is held in memory so its connection can go back to the pool.)
4. An open circuit breaker (`ErrCircuitOpen`) and the caller's own canceled or expired context are never retried, even when a custom `RetryOn` returns `true`.
5. Every retry increments `app_http_retry_count` with a `reason` label (`status_5xx`, `status_429`, `status_408`, `network`, `attempt_timeout`, or `status_other` for any other status a custom `RetryOn` retries), records the wait in `app_http_retry_wait_seconds`, adds an `http.retry` event to the caller's span, and is logged at debug level. A retry refused by the budget increments `app_http_retry_budget_exhausted_total` instead.

### File-Based Token Authentication

`FileTokenAuthConfig` (in `gofr.dev/pkg/gofr/service`) authenticates outgoing
HTTP calls with a bearer token read from a file, re-reading it on a configurable
interval so rotated tokens are picked up without restarting the process. The common
use case is a Kubernetes [projected service account token](https://kubernetes.io/docs/tasks/configure-pod-container/configure-service-account/#serviceaccount-token-volume-projection),
mounted by default at `/var/run/secrets/kubernetes.io/serviceaccount/token`.

The constructor is zero-config for the common K8s case — `service.NewFileTokenAuthConfig()`
reads from the default mount path and refreshes every 30s. Override either via
functional options: `service.WithTokenFilePath(path)` and
`service.WithRefreshInterval(d)`. The logger used to report background refresh
failures at WARN level is injected automatically by `AddHTTPService` — you do
not have to plumb it through. The cached token keeps serving until the next
successful read.

```go
package main

import (
	"gofr.dev/pkg/gofr"
	"gofr.dev/pkg/gofr/service"
)

func main() {
	app := gofr.New()

	tokenCfg, err := service.NewFileTokenAuthConfig()
	if err != nil {
		app.Logger().Fatalf("failed to initialize file token auth: %v", err)
	}

	app.AddHTTPService("upstream", "https://example.com", tokenCfg)

	app.Run()
}
```

To override defaults:

```go
tokenCfg, err := service.NewFileTokenAuthConfig(
    service.WithTokenFilePath("/var/run/custom/token"),
    service.WithRefreshInterval(15*time.Second),
)
```

`tokenCfg` runs a background refresh goroutine for the lifetime of the process;
registering it with `AddHTTPService` keeps it alive until the app exits, so there
is no need to call `tokenCfg.Close()`. Call `Close()` only if you construct a config
that outlives the service using it.

## Metrics

GoFr publishes the following metrics for HTTP service communication:

- `app_http_retry_count`: Total number of retry events. (labels: `service`; with `RetryPolicy` also `reason`)
- `app_http_retry_wait_seconds`: Time `RetryPolicy` waited before each retry, in seconds (histogram). (labels: `service`)
- `app_http_retry_budget_exhausted_total`: Retries `RetryPolicy` skipped because the retry budget was spent. (labels: `service`)
- `app_http_circuit_breaker_state`: Current state of the circuit breaker (0 for Closed, 1 for Open). (labels: `service`)
- `app_http_service_response`: Response time of HTTP service requests in seconds (histogram). (labels: `service`, `path`, `method`, `status`)
## Related production guides

- **Distributed Tracing**: [Trace inter-service HTTP calls end-to-end](/docs/guides/distributed-tracing) — context propagation across GoFr's HTTP client.
