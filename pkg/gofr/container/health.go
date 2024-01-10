package container

type HealthChecker interface {
	HealthCheck() Health
}

type Health struct {
	Name    string
	Status  string
	Type    string
	Details interface{}
}
