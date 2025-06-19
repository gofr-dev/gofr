package gofr

import "time"

const (
	defaultPublicStaticDir = "static"
	shutDownTimeout        = 30 * time.Second
	gofrTraceExporter      = "gofr"
	gofrTracerURL          = "https://tracer.gofr.dev"
	checkPortTimeout       = 2 * time.Second
	gofrHost               = "https://gofr.dev"
	startServerPing        = "/api/ping/up"
	shutServerPing         = "/api/ping/down"
	pingTimeout            = 5 * time.Second
	defaultTelemetry       = "true"
	defaultReflection      = "false"
)
