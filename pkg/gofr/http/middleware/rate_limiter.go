package middleware

import (
	"context"
	"errors"
	"fmt"
	"math"
	"net"
	"net/http"
	"strings"

	gofrHttp "gofr.dev/pkg/gofr/http"
)

var (
	// ErrInvalidRequestsPerSecond is returned when RequestsPerSecond is not positive.
	ErrInvalidRequestsPerSecond = errors.New("requestsPerSecond must be positive")

	// ErrInvalidBurst is returned when Burst is not positive.
	ErrInvalidBurst = errors.New("burst must be positive")
)

// RateLimiterConfig holds configuration for rate limiting.
//
// Note: The default implementation uses in-memory token buckets and is suitable
// for single-pod deployments. In multi-pod deployments, each pod will enforce
// limits independently. For distributed rate limiting across multiple pods,
// a Redis-backed store can be implemented in a future update.
//
// Security: When using PerIP=true, only enable TrustedProxies if your application
// is behind a trusted reverse proxy (nginx, ALB, etc.) that sets X-Forwarded-For.
// Without trusted proxies, clients can spoof IP addresses to bypass rate limits.
//
// Cleanup: The rate limiter starts a background goroutine that runs for the
// application lifetime. This is acceptable for long-running servers but consider
// calling Store.StopCleanup() in shutdown handlers if needed.
type RateLimiterConfig struct {
	RequestsPerSecond float64
	Burst             int
	PerIP             bool
	Store             RateLimiterStore // Optional: defaults to in-memory store
	TrustedProxies    bool             // If true, trust X-Forwarded-For and X-Real-IP headers
}

// Validate checks if the configuration values are valid.
func (c RateLimiterConfig) Validate() error {
	if c.RequestsPerSecond <= 0 {
		return ErrInvalidRequestsPerSecond
	}

	if c.Burst <= 0 {
		return ErrInvalidBurst
	}

	return nil
}

type rateLimiterMetrics interface {
	IncrementCounter(ctx context.Context, name string, labels ...string)
}

// getIP extracts the client IP address from the request.
// If trustProxies is false, only RemoteAddr is used to prevent IP spoofing.
func getIP(r *http.Request, trustProxies bool) string {
	// Only trust proxy headers if explicitly configured
	if trustProxies {
		// Check X-Forwarded-For header first
		forwarded := r.Header.Get("X-Forwarded-For")
		if forwarded != "" {
			// X-Forwarded-For can contain multiple IPs, take the first one
			ips := strings.Split(forwarded, ",")
			if len(ips) > 0 {
				return strings.TrimSpace(ips[0])
			}
		}

		// Check X-Real-IP header
		if realIP := r.Header.Get("X-Real-IP"); realIP != "" {
			return realIP
		}
	}

	// Fall back to RemoteAddr (always used if not trusting proxies)
	ip, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}

	return ip
}

// RateLimiter creates a middleware that limits requests based on the configuration.
func RateLimiter(config RateLimiterConfig, metrics rateLimiterMetrics) func(http.Handler) http.Handler {
	// Validate configuration
	if err := config.Validate(); err != nil {
		panic(fmt.Sprintf("invalid rate limiter config: %v", err))
	}

	// Use in-memory store if none provided
	if config.Store == nil {
		config.Store = NewMemoryRateLimiterStore(config)
	}

	// Start cleanup routine with context.Background().
	// The cleanup goroutine runs for the application lifetime.
	// For graceful shutdown, call config.Store.StopCleanup() in your shutdown handler.
	ctx := context.Background()
	config.Store.StartCleanup(ctx)

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Skip rate limiting for health check endpoints
			if isWellKnown(r.URL.Path) {
				next.ServeHTTP(w, r)
				return
			}

			// Determine the rate limit key (IP or global)
			key := "global"
			if config.PerIP {
				key = getIP(r, config.TrustedProxies)
			}

			// Check rate limit
			allowed, retryAfter, err := config.Store.Allow(r.Context(), key, config)
			if err != nil {
				// Fail open on errors
				next.ServeHTTP(w, r)
				return
			}

			if !allowed {
				// Set Retry-After header (RFC 6585)
				// Use math.Ceil to ensure at least 1 second for sub-second delays
				w.Header().Set("Retry-After", fmt.Sprintf("%.0f", math.Ceil(retryAfter.Seconds())))

				// Increment rate limit exceeded metric
				if metrics != nil {
					metrics.IncrementCounter(r.Context(), "app_http_rate_limit_exceeded_total",
						"path", r.URL.Path, "method", r.Method)
				}

				// Return 429 Too Many Requests
				responder := gofrHttp.NewResponder(w, r.Method)
				responder.Respond(nil, gofrHttp.ErrorTooManyRequests{})

				return
			}

			next.ServeHTTP(w, r)
		})
	}
}
