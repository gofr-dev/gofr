package container

type HealthChecker interface {
	HealthCheck() interface{}
}
