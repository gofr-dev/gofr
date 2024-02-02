package service

type HealthConfig struct {
	HealthEndpoint string
}

func (h *HealthConfig) addOption(svc HTTPService) HTTPService {
	return &customHealthService{
		healthEndpoint: h.HealthEndpoint,
		HTTPService:    svc,
	}
}

type customHealthService struct {
	healthEndpoint string
	HTTPService
}

func (c *customHealthService) HealthCheck() *Health {
	return c.HTTPService.getHealthResponseForEndpoint(c.healthEndpoint)
}
