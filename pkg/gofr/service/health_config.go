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

	return &customHealthService{
		healthEndpoint: h.HealthEndpoint,
		timeout:        h.Timeout,
		HTTP:           svc,
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
