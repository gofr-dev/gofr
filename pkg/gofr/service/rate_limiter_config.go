package service

import (
	"errors"
	"fmt"
	"net/http"
	"time"
)

var (
	errInvalidRequestRate     = errors.New("requests must be greater than 0 per configured time window")
	errBurstLessThanRequests  = errors.New("burst must be greater than requests per window")
	errInvalidRedisResultType = errors.New("unexpected Redis result type")
)

const (
	unknownServiceKey = "unknown"
	methodHTTP        = "http"
	methodHTTPS       = "https"
)

// RateLimiterConfig with custom keying support.
type RateLimiterConfig struct {
	Requests float64                    // Number of requests allowed
	Window   time.Duration              // Time window (e.g., time.Minute, time.Hour)
	Burst    int                        // Maximum burst capacity (must be > 0)
	KeyFunc  func(*http.Request) string // Optional custom key extraction
	Store    RateLimiterStore
}

// defaultKeyFunc extracts a normalized service key from an HTTP request.
func defaultKeyFunc(req *http.Request) string {
	if req == nil || req.URL == nil {
		return unknownServiceKey
	}

	scheme := req.URL.Scheme
	host := req.URL.Host

	if scheme == "" {
		if req.TLS != nil {
			scheme = methodHTTPS
		} else {
			scheme = methodHTTP
		}
	}

	if host == "" {
		host = req.Host
	}

	if host == "" {
		host = unknownServiceKey
	}

	return scheme + "://" + host
}

// Validate checks if the configuration is valid.
// Validate checks if the configuration is valid and sets defaults.
func (config *RateLimiterConfig) Validate() error {
	var validationError error

	if config.Requests <= 0 {
		validationError = fmt.Errorf("%w: %f", errInvalidRequestRate, config.Requests)

		config.Requests = 60 // Default: 60 requests per minute
	}

	if config.Window <= 0 {
		config.Window = defaultWindow
	}

	if config.Burst <= 0 {
		config.Burst = defaultBurstCapacity // Default: allow burst of 10 requests
	}

	if float64(config.Burst) < config.Requests {
		validationError = fmt.Errorf("%w: burst=%d, requests=%f", errBurstLessThanRequests, config.Burst, config.Requests)

		config.Burst = int(config.Requests)
	}

	// Set default key function if not provided
	if config.KeyFunc == nil {
		config.KeyFunc = defaultKeyFunc
	}

	return validationError
}

// RequestsPerSecond converts the configured rate to requests per second.
func (config *RateLimiterConfig) RequestsPerSecond() float64 {
	// Convert any time window to "requests per second" for internal math
	return float64(config.Requests) / config.Window.Seconds()
}

// RateLimitError represents a rate limiting error.
type RateLimitError struct {
	ServiceKey string
	RetryAfter time.Duration
}

func (e *RateLimitError) Error() string {
	return fmt.Sprintf("rate limit exceeded for service: %s, retry after: %v", e.ServiceKey, e.RetryAfter)
}

// StatusCode Implement StatusCodeResponder so Responder picks correct HTTP code.
func (*RateLimitError) StatusCode() int {
	return http.StatusTooManyRequests // 429
}
