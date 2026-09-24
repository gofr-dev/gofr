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
	"gofr.dev/pkg/gofr/logging"
)

var (
	// errInvalidRequestsPerSecond is returned when RequestsPerSecond is not positive (or is NaN).
	errInvalidRequestsPerSecond = errors.New("requestsPerSecond must be positive")

	// errInvalidBurst is returned when Burst is not positive.
	errInvalidBurst = errors.New("burst must be positive")
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
	// MaxKeys bounds the unique rate limit keys of the default in-memory store
	// (0 or negative = default 100000). A caller-supplied Store owns its own bound.
	MaxKeys int64
}

// Validate checks if the configuration values are valid.
//
// RequestsPerSecond must be positive and not NaN (a NaN rate would let every request
// through silently). +Inf is allowed and behaves as unlimited. Burst must be positive.
//
// A zero or negative MaxKeys is not an error: when GoFr builds the default in-memory
// store it falls back to the default key limit. A caller-supplied Store is responsible
// for its own bound.
func (c RateLimiterConfig) Validate() error {
	if c.RequestsPerSecond <= 0 || math.IsNaN(c.RequestsPerSecond) {
		return errInvalidRequestsPerSecond
	}

	if c.Burst <= 0 {
		return errInvalidBurst
	}

	return nil
}

// getIP extracts the client IP address from the request.
// If trustProxies is false, only RemoteAddr is used to prevent IP spoofing.
func getIP(r *http.Request, trustProxies bool) string {
	if !trustProxies {
		return getRemoteAddr(r)
	}

	// Try X-Forwarded-For header first
	if ip := getForwardedIP(r); ip != "" {
		return ip
	}

	// Try X-Real-IP header
	if ip := getRealIP(r); ip != "" {
		return ip
	}

	// Fall back to RemoteAddr
	return getRemoteAddr(r)
}

// getForwardedIP extracts IP from X-Forwarded-For header.
func getForwardedIP(r *http.Request) string {
	forwarded := r.Header.Get("X-Forwarded-For")
	if forwarded == "" {
		return ""
	}

	// X-Forwarded-For can contain multiple IPs, take the first one
	ips := strings.Split(forwarded, ",")
	if len(ips) == 0 {
		return ""
	}

	return strings.TrimSpace(ips[0])
}

// getRealIP extracts IP from X-Real-IP header.
func getRealIP(r *http.Request) string {
	realIP := r.Header.Get("X-Real-IP")
	return strings.TrimSpace(realIP)
}

// getRemoteAddr extracts IP from RemoteAddr.
func getRemoteAddr(r *http.Request) string {
	ip, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}

	return ip
}

// RateLimiterOption configures the RateLimiter middleware.
type RateLimiterOption func(*rateLimiterOptions)

type rateLimiterOptions struct {
	logger rateLimiterLogger
}

// rateLimiterLogger is the logging contract RateLimiter needs; app.Logger() satisfies it.
type rateLimiterLogger interface {
	Errorf(format string, args ...any)
}

// WithRateLimiterLogger sets the logger RateLimiter uses to report an invalid
// RateLimiterConfig and a negative MaxKeys that falls back to the default key limit.
// l is any value with an Errorf(format string, args ...any) method,
// typically app.Logger(). Without this option, or when l is nil, the error is still
// logged to stderr by a GoFr logger.
func WithRateLimiterLogger(l rateLimiterLogger) RateLimiterOption {
	return func(o *rateLimiterOptions) {
		if l != nil {
			o.logger = l
		}
	}
}

// errorLogger returns the configured logger, falling back to a GoFr stderr logger when
// none was provided so a misconfiguration is never silent.
func (o *rateLimiterOptions) errorLogger() rateLimiterLogger {
	if o.logger == nil {
		o.logger = logging.NewLogger(logging.ERROR)
	}

	return o.logger
}

// logInvalidConfig reports an invalid config at ERROR level.
func (o *rateLimiterOptions) logInvalidConfig(err error) {
	o.errorLogger().Errorf("invalid rate limiter config: %v; rate limiting is disabled", err)
}

// logMaxKeysFallback reports a negative MaxKeys that the default in-memory store replaces
// with its default key limit.
func (o *rateLimiterOptions) logMaxKeysFallback(maxKeys int64) {
	o.errorLogger().Errorf("invalid rate limiter MaxKeys %d; falling back to the default key limit", maxKeys)
}

// storeFor returns config.Store, or builds the default in-memory store when none is
// provided. That store replaces a negative MaxKeys with its default, which is logged.
func (o *rateLimiterOptions) storeFor(config RateLimiterConfig) RateLimiterStore {
	if config.Store != nil {
		return config.Store
	}

	if config.MaxKeys < 0 {
		o.logMaxKeysFallback(config.MaxKeys)
	}

	return NewMemoryRateLimiterStore(config)
}

// RateLimiter creates a middleware that limits requests based on the configuration.
//
// If the configuration is invalid (see RateLimiterConfig.Validate), the error is logged
// and the returned middleware passes every request through without rate limiting.
// Pass WithRateLimiterLogger(app.Logger()) to report it through the app's logger, or
// call config.Validate() at startup to fail fast instead.
func RateLimiter(config RateLimiterConfig, m metrics, opts ...RateLimiterOption) func(http.Handler) http.Handler {
	var options rateLimiterOptions

	for _, opt := range opts {
		opt(&options)
	}

	if err := config.Validate(); err != nil {
		options.logInvalidConfig(err)

		return func(next http.Handler) http.Handler { return next }
	}

	config.Store = options.storeFor(config)

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
				// Fallback to "unknown" if getIP returns empty string
				// This prevents all requests from sharing the same bucket
				if key == "" {
					key = "unknown"
				}
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
				if m != nil {
					m.IncrementCounter(r.Context(), "app_http_rate_limit_exceeded_total",
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
