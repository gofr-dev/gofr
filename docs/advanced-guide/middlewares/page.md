# Middleware in GoFr

Middleware allows you intercepting and manipulating HTTP requests and responses flowing through your application's
router. Middlewares can perform tasks such as authentication, authorization, caching etc. before
or after the request reaches your application's handler.

## CORS Middleware in GoFr
GoFr includes built-in CORS (Cross-Origin Resource Sharing) middleware to handle CORS-related headers. 
This middleware allows you to control access to your API from different origins. It automatically adds the necessary
headers to responses, allowing or restricting cross-origin requests. User can also override the default response headers
sent by GoFr by providing the suitable CORS configs.

The CORS middleware provides the following overridable configs:

- `ACCESS_CONTROL_ALLOW_ORIGIN`: Set the allowed origin(s) for cross-origin requests. By default, it allows all origins (*).
- `ACCESS_CONTROL_ALLOW_HEADERS`: Define the allowed request headers (e.g., Authorization, Content-Type).
- `ACCESS_CONTROL_ALLOW_CREDENTIALS`: Set to true to allow credentials (cookies, HTTP authentication) in requests.
- `ACCESS_CONTROL_EXPOSE_HEADERS`: Specify additional headers exposed to the client.
- `ACCESS_CONTROL_MAX_AGE`: Set the maximum time (in seconds) for preflight request caching.

> Note: GoFr automatically interprets the registered route methods and based on that sets the value of `ACCESS_CONTROL_ALLOW_METHODS`


## Adding Custom Middleware in GoFr

By adding custom middleware to your GoFr application, user can easily extend its functionality and implement 
cross-cutting concerns in a modular and reusable way.
User can use the `UseMiddleware` or `UseMiddlewareWithContainer` method on your GoFr application instance to register your custom middleware.

### Using UseMiddleware method for Custom Middleware
The UseMiddleware method is ideal for simple middleware that doesn't need direct access to the application's container.

#### Example:

```go
import (
	"net/http"

	gofrHTTP "gofr.dev/pkg/gofr/http"
)

// Define your custom middleware function
func customMiddleware() gofrHTTP.Middleware {
	return func(inner http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Your custom logic here
			// For example, logging, authentication, etc.

			// Call the next handler in the chain
			inner.ServeHTTP(w, r)
		})
	}
}

func main() {
	// Create a new instance of your GoFr application
	app := gofr.New()

	// Add your custom middleware to the application
	app.UseMiddleware(customMiddleware())

	// Define your application routes and handlers
	// ...

	// Run your GoFr application
	app.Run()
}
```

### Using UseMiddlewareWithContainer method for Custom Middleware

The `UseMiddlewareWithContainer` method is designed for middleware that requires access to GoFr's container, 
which provides access to databases, loggers, metrics, and other application resources. This is particularly 
useful for middleware that needs to interact with datastores or perform logging.

#### Example:

```go
import (
	"net/http"

	"gofr.dev/pkg/gofr"
	"gofr.dev/pkg/gofr/container"
	gofrHTTP "gofr.dev/pkg/gofr/http"
)

// Define middleware that needs container access
func databaseMiddleware(c *container.Container) gofrHTTP.Middleware {
	return func(inner http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Access container resources like database, logger, etc.
			c.Logger.Info("Processing request with database middleware")

			// You can perform database operations, cache checks, etc.
			// For example: validate API key from database
			// apiKey := r.Header.Get("X-API-Key")
			// valid, err := validateAPIKeyFromDB(c, apiKey)

			// Call the next handler in the chain
			inner.ServeHTTP(w, r)
		})
	}
}

func main() {
	app := gofr.New()

	// Add middleware that needs container access
	app.UseMiddlewareWithContainer(databaseMiddleware)

	// Define your application routes and handlers
	// ...

	app.Run()
}
```

## Advanced Middleware Examples

### 1. Rate Limiting Middleware

Rate limiting helps protect your API from abuse by limiting the number of requests a client can make within a time window.
This example implements a simple token bucket algorithm.

```go
import (
	"net/http"
	"sync"
	"time"

	gofrHTTP "gofr.dev/pkg/gofr/http"
)

type rateLimiter struct {
	tokens         int
	maxTokens      int
	refillRate     time.Duration
	lastRefillTime time.Time
	mu             sync.Mutex
}

func newRateLimiter(maxTokens int, refillRate time.Duration) *rateLimiter {
	return &rateLimiter{
		tokens:         maxTokens,
		maxTokens:      maxTokens,
		refillRate:     refillRate,
		lastRefillTime: time.Now(),
	}
}

func (rl *rateLimiter) allow() bool {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	// Refill tokens based on time elapsed
	now := time.Now()
	elapsed := now.Sub(rl.lastRefillTime)
	tokensToAdd := int(elapsed / rl.refillRate)

	if tokensToAdd > 0 {
		rl.tokens = min(rl.maxTokens, rl.tokens+tokensToAdd)
		rl.lastRefillTime = now
	}

	// Check if request can proceed
	if rl.tokens > 0 {
		rl.tokens--
		return true
	}

	return false
}

func rateLimitMiddleware(maxRequests int, window time.Duration) gofrHTTP.Middleware {
	limiters := make(map[string]*rateLimiter)
	var mu sync.Mutex

	return func(inner http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Use IP address as identifier (you can use API key, user ID, etc.)
			clientIP := r.RemoteAddr

			mu.Lock()
			limiter, exists := limiters[clientIP]
			if !exists {
				limiter = newRateLimiter(maxRequests, window/time.Duration(maxRequests))
				limiters[clientIP] = limiter
			}
			mu.Unlock()

			if !limiter.allow() {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusTooManyRequests)
				w.Write([]byte(`{"error": "rate limit exceeded"}`))
				return
			}

			inner.ServeHTTP(w, r)
		})
	}
}

// Usage
func main() {
	app := gofr.New()

	// Allow 100 requests per minute per client
	app.UseMiddleware(rateLimitMiddleware(100, time.Minute))

	app.GET("/api/resource", handler)
	app.Run()
}
```

### 2. JWT Authentication Middleware

This middleware validates JWT tokens and extracts claims for use in your handlers. It demonstrates custom authentication
beyond the built-in OAuth support.

```go
import (
	"context"
	"net/http"
	"strings"

	"github.com/golang-jwt/jwt/v5"
	gofrHTTP "gofr.dev/pkg/gofr/http"
)

type contextKey string

const userClaimsKey contextKey = "userClaims"

type Claims struct {
	UserID string `json:"user_id"`
	Email  string `json:"email"`
	Role   string `json:"role"`
	jwt.RegisteredClaims
}

func jwtAuthMiddleware(secretKey string) gofrHTTP.Middleware {
	return func(inner http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Extract token from Authorization header
			authHeader := r.Header.Get("Authorization")
			if authHeader == "" {
				http.Error(w, "missing authorization header", http.StatusUnauthorized)
				return
			}

			// Expected format: "Bearer <token>"
			parts := strings.Split(authHeader, " ")
			if len(parts) != 2 || parts[0] != "Bearer" {
				http.Error(w, "invalid authorization header format", http.StatusUnauthorized)
				return
			}

			tokenString := parts[1]

			// Parse and validate token
			claims := &Claims{}
			token, err := jwt.ParseWithClaims(tokenString, claims, func(token *jwt.Token) (interface{}, error) {
				// Validate signing method
				if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
					return nil, jwt.ErrSignatureInvalid
				}
				return []byte(secretKey), nil
			})

			if err != nil || !token.Valid {
				http.Error(w, "invalid or expired token", http.StatusUnauthorized)
				return
			}

			// Add claims to request context for use in handlers
			ctx := context.WithValue(r.Context(), userClaimsKey, claims)
			inner.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// Helper function to extract claims from context in your handlers
func GetUserClaims(ctx context.Context) (*Claims, bool) {
	claims, ok := ctx.Value(userClaimsKey).(*Claims)
	return claims, ok
}

// Usage
func main() {
	app := gofr.New()

	// Add JWT authentication middleware
	app.UseMiddleware(jwtAuthMiddleware("your-secret-key"))

	app.GET("/protected", func(c *gofr.Context) (interface{}, error) {
		// Extract user claims from context
		claims, ok := GetUserClaims(c.Request.Context())
		if !ok {
			return nil, errors.New("failed to get user claims")
		}

		return map[string]string{
			"message": "authenticated",
			"user_id": claims.UserID,
			"role":    claims.Role,
		}, nil
	})

	app.Run()
}
```

### 3. Security Headers Middleware

This middleware adds essential security headers to protect your application from common web vulnerabilities.

```go
import (
	"net/http"

	gofrHTTP "gofr.dev/pkg/gofr/http"
)

func securityHeadersMiddleware() gofrHTTP.Middleware {
	return func(inner http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Prevent clickjacking attacks
			w.Header().Set("X-Frame-Options", "DENY")

			// Enable browser XSS protection
			w.Header().Set("X-XSS-Protection", "1; mode=block")

			// Prevent MIME type sniffing
			w.Header().Set("X-Content-Type-Options", "nosniff")

			// Enforce HTTPS (adjust max-age as needed)
			w.Header().Set("Strict-Transport-Security", "max-age=31536000; includeSubDomains")

			// Content Security Policy - adjust based on your needs
			w.Header().Set("Content-Security-Policy", "default-src 'self'; script-src 'self'; style-src 'self' 'unsafe-inline'")

			// Referrer Policy
			w.Header().Set("Referrer-Policy", "strict-origin-when-cross-origin")

			// Permissions Policy (formerly Feature-Policy)
			w.Header().Set("Permissions-Policy", "geolocation=(), microphone=(), camera=()")

			inner.ServeHTTP(w, r)
		})
	}
}

// Usage
func main() {
	app := gofr.New()

	// Add security headers to all responses
	app.UseMiddleware(securityHeadersMiddleware())

	app.GET("/api/data", handler)
	app.Run()
}
```

### 4. Request Validation Middleware

This middleware validates incoming requests to ensure they meet your API requirements before reaching your handlers.

```go
import (
	"encoding/json"
	"net/http"
	"strings"

	gofrHTTP "gofr.dev/pkg/gofr/http"
)

type validationConfig struct {
	requiredHeaders  []string
	allowedMethods   []string
	maxBodySize      int64
	requireJSON      bool
}

func requestValidationMiddleware(config validationConfig) gofrHTTP.Middleware {
	return func(inner http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Validate HTTP method
			if len(config.allowedMethods) > 0 {
				methodAllowed := false
				for _, method := range config.allowedMethods {
					if r.Method == method {
						methodAllowed = true
						break
					}
				}
				if !methodAllowed {
					http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
					return
				}
			}

			// Validate required headers
			for _, header := range config.requiredHeaders {
				if r.Header.Get(header) == "" {
					w.WriteHeader(http.StatusBadRequest)
					json.NewEncoder(w).Encode(map[string]string{
						"error": "missing required header: " + header,
					})
					return
				}
			}

			// Validate Content-Type for JSON requests
			if config.requireJSON && (r.Method == "POST" || r.Method == "PUT" || r.Method == "PATCH") {
				contentType := r.Header.Get("Content-Type")
				if !strings.Contains(contentType, "application/json") {
					http.Error(w, "content-type must be application/json", http.StatusUnsupportedMediaType)
					return
				}
			}

			// Validate body size
			if config.maxBodySize > 0 && r.ContentLength > config.maxBodySize {
				http.Error(w, "request body too large", http.StatusRequestEntityTooLarge)
				return
			}

			inner.ServeHTTP(w, r)
		})
	}
}

// Usage
func main() {
	app := gofr.New()

	// Add request validation middleware
	app.UseMiddleware(requestValidationMiddleware(validationConfig{
		requiredHeaders: []string{"X-API-Version"},
		allowedMethods:  []string{"GET", "POST", "PUT", "DELETE"},
		maxBodySize:     1024 * 1024, // 1MB
		requireJSON:     true,
	}))

	app.POST("/api/users", createUserHandler)
	app.Run()
}
```

### 5. Enhanced Logging Middleware

While GoFr includes built-in logging middleware, you can create custom logging middleware for specific needs,
such as logging request/response bodies or custom metrics.

```go
import (
	"bytes"
	"io"
	"net/http"
	"time"

	"gofr.dev/pkg/gofr/container"
	gofrHTTP "gofr.dev/pkg/gofr/http"
)

type responseWriter struct {
	http.ResponseWriter
	statusCode int
	body       *bytes.Buffer
}

func (rw *responseWriter) WriteHeader(code int) {
	rw.statusCode = code
	rw.ResponseWriter.WriteHeader(code)
}

func (rw *responseWriter) Write(b []byte) (int, error) {
	rw.body.Write(b)
	return rw.ResponseWriter.Write(b)
}

func customLoggingMiddleware(c *container.Container) gofrHTTP.Middleware {
	return func(inner http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()

			// Read and restore request body for logging
			var requestBody []byte
			if r.Body != nil {
				requestBody, _ = io.ReadAll(r.Body)
				r.Body = io.NopCloser(bytes.NewBuffer(requestBody))
			}

			// Wrap response writer to capture response
			rw := &responseWriter{
				ResponseWriter: w,
				statusCode:     http.StatusOK,
				body:           &bytes.Buffer{},
			}

			// Process request
			inner.ServeHTTP(rw, r)

			// Log request details
			duration := time.Since(start)
			c.Logger.Infof("method=%s path=%s status=%d duration=%v request_size=%d response_size=%d",
				r.Method,
				r.URL.Path,
				rw.statusCode,
				duration,
				len(requestBody),
				rw.body.Len(),
			)

			// Optionally log request/response bodies for debugging (be careful with sensitive data)
			if c.Config.Get("LOG_REQUEST_BODY") == "true" {
				c.Logger.Debugf("request_body=%s", string(requestBody))
			}
		})
	}
}

// Usage
func main() {
	app := gofr.New()

	// Add custom logging middleware with container access
	app.UseMiddlewareWithContainer(customLoggingMiddleware)

	app.POST("/api/data", handler)
	app.Run()
}
```

### 6. Timeout Middleware

This middleware enforces request timeouts to prevent long-running requests from consuming resources.

```go
import (
	"context"
	"net/http"
	"time"

	gofrHTTP "gofr.dev/pkg/gofr/http"
)

func timeoutMiddleware(timeout time.Duration) gofrHTTP.Middleware {
	return func(inner http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Create a context with timeout
			ctx, cancel := context.WithTimeout(r.Context(), timeout)
			defer cancel()

			// Channel to signal completion
			done := make(chan struct{})

			// Run handler in goroutine
			go func() {
				inner.ServeHTTP(w, r.WithContext(ctx))
				close(done)
			}()

			// Wait for completion or timeout
			select {
			case <-done:
				// Request completed successfully
				return
			case <-ctx.Done():
				// Timeout occurred
				if ctx.Err() == context.DeadlineExceeded {
					http.Error(w, "request timeout", http.StatusGatewayTimeout)
				}
			}
		})
	}
}

// Usage
func main() {
	app := gofr.New()

	// Set 30-second timeout for all requests
	app.UseMiddleware(timeoutMiddleware(30 * time.Second))

	app.GET("/api/long-running", handler)
	app.Run()
}
```

## Best Practices

### Middleware Ordering

The order in which middleware is registered matters. Middleware executes in the order it's added, and each middleware
wraps the next one in the chain. Follow this recommended order:

1. **Recovery/Panic handling** - Should be first to catch panics from any middleware
2. **Logging** - Early logging captures all requests
3. **Security headers** - Apply security policies early
4. **CORS** - Handle preflight requests before authentication
5. **Authentication** - Verify identity before authorization
6. **Authorization** - Check permissions after authentication
7. **Rate limiting** - Prevent abuse from authenticated users
8. **Request validation** - Validate after authentication but before business logic
9. **Timeout** - Wrap business logic with timeout
10. **Business logic handlers** - Your actual route handlers

Example:
```go
app := gofr.New()

// Recommended order
app.UseMiddleware(recoveryMiddleware())
app.UseMiddleware(loggingMiddleware())
app.UseMiddleware(securityHeadersMiddleware())
// CORS is built-in and automatically applied
app.UseMiddleware(authenticationMiddleware())
app.UseMiddleware(rateLimitMiddleware(100, time.Minute))
app.UseMiddleware(requestValidationMiddleware(config))
app.UseMiddleware(timeoutMiddleware(30 * time.Second))

// Register routes
app.GET("/api/resource", handler)
```

### Error Handling

Middleware should handle errors gracefully and provide meaningful responses:

```go
func errorHandlingMiddleware() gofrHTTP.Middleware {
	return func(inner http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			defer func() {
				if err := recover(); err != nil {
					// Log the error
					// Return appropriate error response
					w.WriteHeader(http.StatusInternalServerError)
					json.NewEncoder(w).Encode(map[string]string{
						"error": "internal server error",
					})
				}
			}()

			inner.ServeHTTP(w, r)
		})
	}
}
```

### Performance Considerations

1. **Avoid heavy operations** - Keep middleware lightweight; defer heavy operations to handlers
2. **Use connection pooling** - When accessing databases or external services in middleware
3. **Cache when possible** - Cache validation results, configuration, etc.
4. **Limit body reading** - Only read request bodies when necessary
5. **Use sync.Pool** - For frequently allocated objects like buffers

### Security Best Practices

1. **Validate all inputs** - Never trust client data
2. **Use HTTPS** - Always enforce HTTPS in production
3. **Implement rate limiting** - Protect against abuse and DDoS
4. **Sanitize logs** - Don't log sensitive data (passwords, tokens, PII)
5. **Set security headers** - Use CSP, HSTS, X-Frame-Options, etc.
6. **Keep secrets secure** - Use environment variables or secret management services

### Testing Middleware

Test middleware in isolation to ensure correct behavior:

```go
func TestRateLimitMiddleware(t *testing.T) {
	middleware := rateLimitMiddleware(2, time.Second)
	
	handler := middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	// Test that first 2 requests succeed
	for i := 0; i < 2; i++ {
		req := httptest.NewRequest("GET", "/test", nil)
		req.RemoteAddr = "127.0.0.1:1234"
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)
		
		if w.Code != http.StatusOK {
			t.Errorf("Expected status 200, got %d", w.Code)
		}
	}

	// Test that 3rd request is rate limited
	req := httptest.NewRequest("GET", "/test", nil)
	req.RemoteAddr = "127.0.0.1:1234"
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)
	
	if w.Code != http.StatusTooManyRequests {
		t.Errorf("Expected status 429, got %d", w.Code)
	}
}
```
