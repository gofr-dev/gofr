package service

import (
	"errors"
	"fmt"
	"net/http"
	"time"

	gofrRedis "gofr.dev/pkg/gofr/datasource/redis"
)

var (
	errInvalidRequestRate     = errors.New("requestsPerSecond must be greater than 0")
	errInvalidBurstSize       = errors.New("burst must be greater than 0")
	errInvalidRedisResultType = errors.New("unexpected Redis result type")
)

// RateLimiterConfig with custom keying support.
type RateLimiterConfig struct {
	RequestsPerSecond float64                    // Token refill rate (must be > 0)
	Burst             int                        // Maximum burst capacity (must be > 0)
	KeyFunc           func(*http.Request) string // Optional custom key extraction
	RedisClient       *gofrRedis.Redis           `json:"-"` // Optional Redis for distributed limiting
}

// Default key function extracts scheme://host
func defaultKeyFunc(req *http.Request) string {
	if req == nil || req.URL == nil {
		return "unknown"
	}

	return req.URL.Scheme + "://" + req.URL.Host
}

// Validate checks if the configuration is valid.
func (config *RateLimiterConfig) Validate() error {
	if config.RequestsPerSecond <= 0 {
		return fmt.Errorf("%w: %f", errInvalidRequestRate, config.RequestsPerSecond)
	}

	if config.Burst <= 0 {
		return fmt.Errorf("%w: %d", errInvalidBurstSize, config.Burst)
	}

	// Set default key function if not provided.
	if config.KeyFunc == nil {
		config.KeyFunc = defaultKeyFunc
	}

	return nil
}

// AddOption implements the Options interface.
func (config *RateLimiterConfig) AddOption(h HTTP) HTTP {
	if err := config.Validate(); err != nil {
		if httpSvc, ok := h.(*httpService); ok {
			httpSvc.Logger.Log("Invalid rate limiter config, disabling rate limiting", "error", err)
		}

		return h
	}

	// Choose implementation based on Redis client availability.
	if config.RedisClient != nil {
		return NewDistributedRateLimiter(*config, h)
	}

	// Log warning for local rate limiting.
	if httpSvc, ok := h.(*httpService); ok {
		httpSvc.Logger.Log("Using local rate limiting - not suitable for multi-instance deployments")
	}

	return NewLocalRateLimiter(*config, h)
}

// RateLimitError represents a rate limiting error.
type RateLimitError struct {
	ServiceKey string
	RetryAfter time.Duration
}

func (e *RateLimitError) Error() string {
	return fmt.Sprintf("rate limit exceeded for service: %s, retry after: %v", e.ServiceKey, e.RetryAfter)
}
