# Middleware in GoFr

Middleware allows you intercepting and manipulating HTTP requests and responses flowing through your application's
router. Middlewares can perform tasks such as authentication, authorization, caching, rate limiting, logging, 
security headers, request validation etc. before or after the request reaches your application's handler.

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

### Using UseMiddlewareWithContainer method for Advanced Middleware
The UseMiddlewareWithContainer method is ideal for middleware that needs access to the application's container
for database connections, configuration, or other services.

#### Example:

```go
import (
	"net/http"
	"gofr.dev/pkg/gofr"
	gofrHTTP "gofr.dev/pkg/gofr/http"
)

// Define middleware that uses container
func authMiddleware() gofrHTTP.MiddlewareWithContainer {
	return func(c *gofr.Container) gofrHTTP.Middleware {
		return func(inner http.Handler) http.Handler {
			return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				// Access database through container
				token := r.Header.Get("Authorization")
				if token == "" {
					http.Error(w, "Unauthorized", http.StatusUnauthorized)
					return
				}

				// Validate token using database
				// isValid := validateTokenFromDB(c.DB, token)
				
				inner.ServeHTTP(w, r)
			})
		}
	}
}

func main() {
	app := gofr.New()
	
	// Add middleware that uses container
	app.UseMiddlewareWithContainer(authMiddleware())
	
	app.Run()
}
```

## Advanced Middleware Examples

### 1. Rate Limiting Middleware

```go
import (
	"net/http"
	"sync"
	"time"
	gofrHTTP "gofr.dev/pkg/gofr/http"
)

type rateLimiter struct {
	visitors map[string]*visitor
	mu       sync.RWMutex
}

type visitor struct {
	lastSeen time.Time
	bucket   int
}

func newRateLimiter() *rateLimiter {
	return &rateLimiter{
		visitors: make(map[string]*visitor),
	}
}

func rateLimitMiddleware(maxRequests int, window time.Duration) gofrHTTP.Middleware {
	limiter := newRateLimiter()
	
	return func(inner http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ip := r.Header.Get("X-Forwarded-For")
			if ip == "" {
				ip = r.RemoteAddr
			}

			limiter.mu.Lock()
			v, exists := limiter.visitors[ip]
			if !exists {
				limiter.visitors[ip] = &visitor{
					lastSeen: time.Now(),
					bucket:   1,
				}
				limiter.mu.Unlock()
				inner.ServeHTTP(w, r)
				return
			}

			if time.Since(v.lastSeen) > window {
				v.bucket = 1
				v.lastSeen = time.Now()
			} else {
				v.bucket++
			}

			if v.bucket > maxRequests {
				limiter.mu.Unlock()
				http.Error(w, "Rate limit exceeded", http.StatusTooManyRequests)
				return
			}
			
			limiter.mu.Unlock()
			inner.ServeHTTP(w, r)
		})
	}
}
```

### 2. Request Logging Middleware

```go
import (
	"log"
	"net/http"
	"time"
	gofrHTTP "gofr.dev/pkg/gofr/http"
)

func loggingMiddleware() gofrHTTP.Middleware {
	return func(inner http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()
			
			// Create a custom ResponseWriter to capture status code
			lrw := &loggingResponseWriter{ResponseWriter: w, statusCode: http.StatusOK}
			
			inner.ServeHTTP(lrw, r)
			
			duration := time.Since(start)
			log.Printf("[%s] %s %s - %d - %v", 
				r.Method, r.RequestURI, r.RemoteAddr, lrw.statusCode, duration)
		})
	}
}

type loggingResponseWriter struct {
	http.ResponseWriter
	statusCode int
}

func (lrw *loggingResponseWriter) WriteHeader(code int) {
	lrw.statusCode = code
	lrw.ResponseWriter.WriteHeader(code)
}
```

### 3. Security Headers Middleware

```go
import (
	"net/http"
	gofrHTTP "gofr.dev/pkg/gofr/http"
)

func securityHeadersMiddleware() gofrHTTP.Middleware {
	return func(inner http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Set security headers
			w.Header().Set("X-Content-Type-Options", "nosniff")
			w.Header().Set("X-Frame-Options", "DENY")
			w.Header().Set("X-XSS-Protection", "1; mode=block")
			w.Header().Set("Strict-Transport-Security", "max-age=31536000; includeSubDomains")
			w.Header().Set("Content-Security-Policy", "default-src 'self'")
			w.Header().Set("Referrer-Policy", "strict-origin-when-cross-origin")
			
			inner.ServeHTTP(w, r)
		})
	}
}
```

### 4. Request Validation Middleware

```go
import (
	"encoding/json"
	"net/http"
	"strings"
	gofrHTTP "gofr.dev/pkg/gofr/http"
)

func requestValidationMiddleware() gofrHTTP.Middleware {
	return func(inner http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Validate Content-Type for POST/PUT requests
			if r.Method == http.MethodPost || r.Method == http.MethodPut {
				contentType := r.Header.Get("Content-Type")
				if !strings.Contains(contentType, "application/json") {
					http.Error(w, "Content-Type must be application/json", http.StatusBadRequest)
					return
				}
				
				// Validate JSON payload
				if r.ContentLength > 0 {
					var temp interface{}
					decoder := json.NewDecoder(r.Body)
					if err := decoder.Decode(&temp); err != nil {
						http.Error(w, "Invalid JSON payload", http.StatusBadRequest)
						return
					}
				}
			}
			
			inner.ServeHTTP(w, r)
		})
	}
}
```

### 5. JWT Authentication Middleware

```go
import (
	"context"
	"net/http"
	"strings"
	"github.com/golang-jwt/jwt/v4"
	gofrHTTP "gofr.dev/pkg/gofr/http"
)

func jwtAuthMiddleware(secretKey []byte) gofrHTTP.Middleware {
	return func(inner http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			authHeader := r.Header.Get("Authorization")
			if authHeader == "" {
				http.Error(w, "Authorization header required", http.StatusUnauthorized)
				return
			}
			
			tokenString := strings.TrimPrefix(authHeader, "Bearer ")
			if tokenString == authHeader {
				http.Error(w, "Bearer token required", http.StatusUnauthorized)
				return
			}
			
			token, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
				return secretKey, nil
			})
			
			if err != nil || !token.Valid {
				http.Error(w, "Invalid token", http.StatusUnauthorized)
				return
			}
			
			// Add user info to context
			if claims, ok := token.Claims.(jwt.MapClaims); ok {
				ctx := context.WithValue(r.Context(), "user", claims)
				r = r.WithContext(ctx)
			}
			
			inner.ServeHTTP(w, r)
		})
	}
}
```

### 6. Request Timeout Middleware

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
			ctx, cancel := context.WithTimeout(r.Context(), timeout)
			defer cancel()
			
			r = r.WithContext(ctx)
			
			done := make(chan bool)
			go func() {
				inner.ServeHTTP(w, r)
				done <- true
			}()
			
			select {
			case <-done:
				return
			case <-ctx.Done():
				http.Error(w, "Request timeout", http.StatusRequestTimeout)
				return
			}
		})
	}
}
```

## Middleware Chaining Example

```go
func main() {
	app := gofr.New()
	
	// Chain multiple middlewares
	app.UseMiddleware(loggingMiddleware())
	app.UseMiddleware(securityHeadersMiddleware())
	app.UseMiddleware(rateLimitMiddleware(100, time.Minute))
	app.UseMiddleware(timeoutMiddleware(30 * time.Second))
	
	// Protected routes with JWT
	protectedRoutes := app.Group("/api/v1")
	protectedRoutes.UseMiddleware(jwtAuthMiddleware([]byte("your-secret-key")))
	protectedRoutes.UseMiddleware(requestValidationMiddleware())
	
	// Define routes
	app.GET("/health", healthHandler)
	protectedRoutes.POST("/users", createUserHandler)
	protectedRoutes.GET("/users", getUsersHandler)
	
	app.Run()
}
```

## Best Practices for Middleware

1. **Order Matters**: Apply middleware in the correct order. Generally:
   - CORS and security headers first
   - Authentication and authorization
   - Rate limiting
   - Request validation
   - Logging
   - Application-specific middleware

2. **Error Handling**: Always handle errors gracefully in middleware and return appropriate HTTP status codes.

3. **Performance**: Keep middleware lightweight and avoid heavy computations that can slow down requests.

4. **Testing**: Write unit tests for your custom middleware to ensure they work correctly.

5. **Documentation**: Document your middleware functions clearly, especially custom ones.

6. **Reusability**: Design middleware to be reusable across different routes and applications.

By leveraging these middleware patterns, you can build robust, secure, and performant GoFr applications with clean separation of concerns.
