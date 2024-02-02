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

func (h *HealthConfig) GetHealthEndpoint() string {
	return h.HealthEndpoint
}
