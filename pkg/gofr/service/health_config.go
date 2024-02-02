package service

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

func (c *customHealthService) HealthCheck() *Health {
	return c.HTTP.getHealthResponseForEndpoint(c.healthEndpoint)
}
