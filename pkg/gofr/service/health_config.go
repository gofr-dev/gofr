package service

import "context"

type HealthConfig struct {
	HealthEndpoint string
}

func (h *HealthConfig) addOption(svc HTTP) HTTP {
	return &customHealthService{
		healthEndpoint: h.HealthEndpoint,
		HTTP:           svc,
	}
}

type customHealthService struct {
	healthEndpoint string
	HTTP
}

func (c *customHealthService) HealthCheck(ctx context.Context) *Health {
	return c.HTTP.getHealthResponseForEndpoint(ctx, c.healthEndpoint)
}
