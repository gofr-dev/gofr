package service

import "context"

type HealthConfig struct {
	HealthEndpoint string
	Timeout        int
}

func (h *HealthConfig) AddOption(svc HTTP) HTTP {
	// if timeout is not provided we set a convenient default timeout.
	if h.Timeout == 0 {
		h.Timeout = defaultTimeout
	}

	// If the service chain contains a circuit breaker, update it to use this health endpoint
	// This ensures the circuit breaker uses the same health endpoint for recovery checks
	updateCircuitBreakerHealthConfig(svc, h.HealthEndpoint, h.Timeout)

	return &customHealthService{
		healthEndpoint: h.HealthEndpoint,
		timeout:        h.Timeout,
		HTTP:           svc,
	}
}

// updateCircuitBreakerHealthConfig traverses the HTTP service chain to ensure that when a HealthConfig is applied, the circuit breaker
// automatically uses the same health endpoint for its recovery checks.
func updateCircuitBreakerHealthConfig(h HTTP, endpoint string, timeout int) {
	switch v := h.(type) {
	case *circuitBreaker:
		v.setHealthConfig(endpoint, timeout)
	case *retryProvider:
		updateCircuitBreakerHealthConfig(v.HTTP, endpoint, timeout)
	case *authProvider:
		updateCircuitBreakerHealthConfig(v.HTTP, endpoint, timeout)
	case *rateLimiter:
		updateCircuitBreakerHealthConfig(v.HTTP, endpoint, timeout)
	case *customHeader:
		updateCircuitBreakerHealthConfig(v.HTTP, endpoint, timeout)
	case *customHealthService:
		updateCircuitBreakerHealthConfig(v.HTTP, endpoint, timeout)
	}
}

type customHealthService struct {
	healthEndpoint string
	timeout        int
	HTTP
}

func (c *customHealthService) HealthCheck(ctx context.Context) *Health {
	return c.HTTP.getHealthResponseForEndpoint(ctx, c.healthEndpoint, c.timeout)
}
