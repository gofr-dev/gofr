package service

const (
	ServiceUp   = "UP"
	ServiceDown = "DOWN"
)

type Health struct {
	Status  string                 `json:"status"`
	Details map[string]interface{} `json:"details"`
}

type CustomHealthConfig struct {
	HealthEndpoint string
}

func (h *CustomHealthConfig) addOption(svc HTTPService) HTTPService {
	return svc
}
