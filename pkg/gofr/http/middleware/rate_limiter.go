package middleware

import (
	"context"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"

	"golang.org/x/time/rate"

	gofrHttp "gofr.dev/pkg/gofr/http"
)

// RateLimiterConfig holds configuration for rate limiting.
type RateLimiterConfig struct {
	RequestsPerSecond float64
	Burst             int
	PerIP             bool
}

type rateLimiter struct {
	limiters sync.Map // map[string]*rate.Limiter for per-IP rate limiting
	global   *rate.Limiter
	config   RateLimiterConfig
	metrics  rateLimiterMetrics
}

type rateLimiterMetrics interface {
	IncrementCounter(ctx context.Context, name string, labels ...string)
}

// NewRateLimiter creates a new rate limiter with the given configuration.
func NewRateLimiter(config RateLimiterConfig, metrics rateLimiterMetrics) *rateLimiter {
	rl := &rateLimiter{
		config:  config,
		metrics: metrics,
	}

	if !config.PerIP {
		rl.global = rate.NewLimiter(rate.Limit(config.RequestsPerSecond), config.Burst)
	}

	// Start cleanup goroutine for per-IP limiters
	if config.PerIP {
		go rl.cleanupStaleEntries()
	}

	return rl
}

// getLimiter returns the appropriate rate limiter for the request.
func (rl *rateLimiter) getLimiter(ip string) *rate.Limiter {
	if !rl.config.PerIP {
		return rl.global
	}

	// Try to get existing limiter
	if limiter, exists := rl.limiters.Load(ip); exists {
		return limiter.(*rate.Limiter)
	}

	// Create new limiter for this IP
	limiter := rate.NewLimiter(rate.Limit(rl.config.RequestsPerSecond), rl.config.Burst)
	rl.limiters.Store(ip, limiter)

	return limiter
}

// cleanupStaleEntries removes inactive limiters every 5 minutes.
func (rl *rateLimiter) cleanupStaleEntries() {
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()

	for range ticker.C {
		rl.limiters.Range(func(key, value interface{}) bool {
			limiter := value.(*rate.Limiter)
			// If limiter has full burst capacity, it hasn't been used recently
			if limiter.Tokens() == float64(rl.config.Burst) {
				rl.limiters.Delete(key)
			}
			return true
		})
	}
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
	limiter := NewRateLimiter(config, metrics)

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Skip rate limiting for health check endpoints
			if isWellKnown(r.URL.Path) {
				next.ServeHTTP(w, r)
				return
			}

			ip := getIP(r)
			rl := limiter.getLimiter(ip)

			if !rl.Allow() {
				// Increment rate limit exceeded metric
				if metrics != nil {
					metrics.IncrementCounter(r.Context(), "app_http_rate_limit_exceeded_total",
						"path", r.URL.Path, "method", r.Method, "ip", ip)
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
