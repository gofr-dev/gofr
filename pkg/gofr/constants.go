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
	helpFlagShort          = "-h"
	helpFlagLong           = "--help"

	// contentTypeJSON is the media type of JSON request and response bodies.
	contentTypeJSON = "application/json"
	// serviceNameAttributeKey is the attribute under which a registered HTTP service records its name.
	serviceNameAttributeKey = "name"
)
