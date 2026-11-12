package main

import (
	"gofr.dev/pkg/gofr"

	// Registers the keyless "gcp" OTLP trace exporter. Enable it by setting
	// TRACE_EXPORTER=gcp (see configs/.env). On Cloud Run this authenticates
	// via the attached service account — no key file, no Collector sidecar.
	_ "gofr.dev/pkg/gofr/traces/exporters/gcp"
)

func main() {
	a := gofr.New()

	a.GET("/hello", func(c *gofr.Context) (any, error) {
		// A custom span, so the trace carries something beyond the automatic
		// HTTP server span.
		span := c.Trace("greet")
		defer span.End()

		return "Hello from Cloud Run!", nil
	})

	a.Run()
}
