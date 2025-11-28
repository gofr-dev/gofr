package middleware

import (
	"context"
	"net"
	"net/http"
	"strings"

	gofrHttp "gofr.dev/pkg/gofr/http"
)

// RateLimiterConfig holds configuration for rate limiting.
//
// Note: The default implementation uses in-memory token buckets and is suitable
// for single-pod deployments. In multi-pod deployments, each pod will enforce
// limits independently. For distributed rate limiting across multiple pods,
// a Redis-backed store can be implemented in a future update.
type RateLimiterConfig struct {
	RequestsPerSecond float64
	Burst             int
	PerIP             bool
	Store             RateLimiterStore // Optional: defaults to in-memory store
}

type rateLimiterMetrics interface {
	IncrementCounter(ctx context.Context, name string, labels ...string)
}

// getIP extracts the client IP address from the request.
func getIP(r *http.Request) string {
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

	// Fall back to RemoteAddr
	ip, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}

	return ip
}

// RateLimiter creates a middleware that limits requests based on the configuration.
func RateLimiter(config RateLimiterConfig, metrics rateLimiterMetrics) func(http.Handler) http.Handler {
	// Use in-memory store if none provided
	if config.Store == nil {
		config.Store = NewMemoryRateLimiterStore()
	}

	// Start cleanup routine
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
				key = getIP(r)
			}

			// Check rate limit
			allowed, retryAfter, err := config.Store.Allow(r.Context(), key, config)
			if err != nil {
				// Fail open on errors
				next.ServeHTTP(w, r)
				return
			}

			if !allowed {
				// Increment rate limit exceeded metric
				if metrics != nil {
					metrics.IncrementCounter(r.Context(), "app_http_rate_limit_exceeded_total",
						"path", r.URL.Path, "method", r.Method, "ip", getIP(r), "retry_after", retryAfter.String())
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
