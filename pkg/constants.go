package pkg

import "gofr.dev/pkg/log"

const (
	StatusUp                 = "UP"
	StatusDown               = "DOWN"
	StatusDegraded           = "DEGRADED"
	DefaultAppName           = "gofr-app"
	DefaultAppVersion        = "dev"
	Framework                = "gofr-" + log.GofrVersion
	PathHealthCheck          = "/.well-known/health-check"
	PathHeartBeat            = "/.well-known/heartbeat"
	PathOpenAPI              = "/.well-known/openapi.json"
	PathSwagger              = "/.well-known/swagger"
	PathSwaggerWithPathParam = "/.well-known/swagger/{name}"
	FrameworkMetricsPrefix   = "zs_"
)
