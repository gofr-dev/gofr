package main

import (
	"gofr.dev/pkg/gofr"

	// Registers the keyless "gcp" OTLP metrics exporter. Enable it by setting
	// METRICS_EXPORTER=gcp (see configs/.env). On Cloud Run this authenticates
	// via the attached service account — no key file.
	_ "gofr.dev/pkg/gofr/metrics/exporters/gcp"
)

const requestsTotal = "requests_total"

func main() {
	a := gofr.New()

	a.Metrics().NewCounter(requestsTotal, "total number of requests handled")

	a.GET("/hello", func(c *gofr.Context) (any, error) {
		c.Metrics().IncrementCounter(c, requestsTotal)

		return "Hello from Cloud Run!", nil
	})

	a.Run()
}
