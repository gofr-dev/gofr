package service

import (
	"net/http"
	"time"
)

// ConnectionPoolConfig holds the configuration for HTTP connection pool settings.
type ConnectionPoolConfig struct {
	// MaxIdleConns controls the maximum number of idle (keep-alive) connections across all hosts.
	// Zero means no limit.
	MaxIdleConns int

	// MaxIdleConnsPerHost controls the maximum idle (keep-alive) connections to keep per-host.
	// If zero, DefaultMaxIdleConnsPerHost is used.
	MaxIdleConnsPerHost int

	// IdleConnTimeout is the maximum amount of time an idle (keep-alive) connection will remain
	// idle before closing itself. Zero means no limit.
	IdleConnTimeout time.Duration
}

// AddOption implements the Options interface to apply connection pool configuration to HTTP service.
func (c *ConnectionPoolConfig) AddOption(h HTTP) HTTP {
	if httpSvc, ok := h.(*httpService); ok {
		// Create a custom transport with connection pool settings
		transport := &http.Transport{
			MaxIdleConns:        c.MaxIdleConns,
			MaxIdleConnsPerHost: c.MaxIdleConnsPerHost,
			IdleConnTimeout:     c.IdleConnTimeout,
		}

		// Apply the custom transport to the HTTP client
		httpSvc.Client.Transport = transport
	}

	return h
}
