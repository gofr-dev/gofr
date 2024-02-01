package service

const (
	ServiceUp   = "UP"
	ServiceDown = "DOWN"
)

type Health struct {
	Status  string                 `json:"status"`
	Details map[string]interface{} `json:"details"`
}

type HealthConfig struct {
	HealthEndpoint string
}

func (h *HealthConfig) addOption(svc HTTPService) HTTPService {
	return svc
}
