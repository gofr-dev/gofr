package service

import (
	"errors"
	"fmt"
	"net/http"
	"time"
)

var (
	errNegativeMaxIdleConns        = errors.New("MaxIdleConns cannot be negative")
	errNegativeMaxIdleConnsPerHost = errors.New("MaxIdleConnsPerHost cannot be negative")
	errNegativeIdleConnTimeout     = errors.New("IdleConnTimeout cannot be negative")
)

// ConnectionPoolConfig holds the configuration for HTTP connection pool settings.
// It customizes the HTTP transport layer to optimize connection reuse for high-frequency requests.
//
// Note: This configuration must be applied first when using multiple options with AddHTTPService,
// as it needs to access the underlying HTTP client transport. If applied after wrapper options
// (CircuitBreaker, Retry, OAuth), it will be silently ignored.
//
// Example:
//
//	app.AddHTTPService("api-service", "https://api.example.com",
//	    &service.ConnectionPoolConfig{
//	        MaxIdleConns:        100,
//	        MaxIdleConnsPerHost: 20,
//	        IdleConnTimeout:     90 * time.Second,
//	    },
//	    &service.CircuitBreakerConfig{...}, // Other options after ConnectionPoolConfig
//	)
type ConnectionPoolConfig struct {
	// MaxIdleConns controls the maximum number of idle (keep-alive) connections across all hosts.
	// Zero means no limit. Negative values will cause validation error.
	// Default (if not set): 100
	MaxIdleConns int

	// MaxIdleConnsPerHost controls the maximum idle (keep-alive) connections to keep per-host.
	// This is the critical setting for microservices making frequent requests to the same host.
	// Zero uses DefaultMaxIdleConnsPerHost (2). Negative values will cause validation error.
	// Default Go value: 2 (which is often insufficient for microservices)
	// Recommended: 10-20 for typical microservices, higher for high-traffic services
	MaxIdleConnsPerHost int

	// IdleConnTimeout is the maximum amount of time an idle (keep-alive) connection will remain
	// idle before closing itself. Zero means no limit. Negative values will cause validation error.
	// Default (if not set): 90 seconds
	IdleConnTimeout time.Duration
}

// Validate checks if the connection pool configuration values are valid.
func (c *ConnectionPoolConfig) Validate() error {
	if c.MaxIdleConns < 0 {
		return fmt.Errorf("%w, got: %d", errNegativeMaxIdleConns, c.MaxIdleConns)
	}

	if c.MaxIdleConnsPerHost < 0 {
		return fmt.Errorf("%w, got: %d", errNegativeMaxIdleConnsPerHost, c.MaxIdleConnsPerHost)
	}

	if c.IdleConnTimeout < 0 {
		return fmt.Errorf("%w, got: %v", errNegativeIdleConnTimeout, c.IdleConnTimeout)
	}

	return nil
}

// AddOption implements the Options interface to apply connection pool configuration to HTTP service.
// It modifies the underlying HTTP client's transport to use optimized connection pool settings.
func (c *ConnectionPoolConfig) AddOption(h HTTP) HTTP {
	// Extract the base httpService from any wrapped service
	httpSvc := extractHTTPService(h)
	if httpSvc == nil {
		// If we can't find the base service, return unchanged
		// This maintains backward compatibility
		return h
	}

	// Validate configuration before applying
	if err := c.Validate(); err != nil {
		// Log the error but don't fail - maintain backward compatibility
		// In production, this would be logged through the service logger
		return h
	}

	// Clone the default transport to preserve important settings like TLS timeouts and proxy configuration
	transport := http.DefaultTransport.(*http.Transport).Clone()

	// Apply connection pool settings
	// Use sensible defaults if not specified
	if c.MaxIdleConns > 0 {
		transport.MaxIdleConns = c.MaxIdleConns
	} else if c.MaxIdleConns == 0 {
		// Set a reasonable default if not specified
		transport.MaxIdleConns = 100
	}

	if c.MaxIdleConnsPerHost > 0 {
		transport.MaxIdleConnsPerHost = c.MaxIdleConnsPerHost
	}
	// Note: If MaxIdleConnsPerHost is 0, Go uses DefaultMaxIdleConnsPerHost (2)

	if c.IdleConnTimeout > 0 {
		transport.IdleConnTimeout = c.IdleConnTimeout
	} else if c.IdleConnTimeout == 0 {
		// Set a reasonable default if not specified
		transport.IdleConnTimeout = 90 * time.Second
	}

	// Apply the custom transport to the HTTP client
	httpSvc.Client.Transport = transport

	return h
}

// extractHTTPService attempts to extract the base *httpService from a potentially wrapped HTTP service.
// It handles the common wrapper types used in the service package.
func extractHTTPService(h HTTP) *httpService {
	// Direct type assertion
	if httpSvc, ok := h.(*httpService); ok {
		return httpSvc
	}

	// Check if it's a circuit breaker wrapper
	if cb, ok := h.(*circuitBreaker); ok {
		return extractHTTPService(cb.HTTP)
	}

	// Check if it's a retry provider wrapper
	if rp, ok := h.(*retryProvider); ok {
		return extractHTTPService(rp.HTTP)
	}

	// Check if it's an auth provider wrapper (OAuth, BasicAuth, APIKey)
	if ap, ok := h.(*authProvider); ok {
		return extractHTTPService(ap.HTTP)
	}

	// Check if it's a custom health service wrapper
	if chs, ok := h.(*customHealthService); ok {
		return extractHTTPService(chs.HTTP)
	}

	// If we can't extract it, return nil
	return nil
}
