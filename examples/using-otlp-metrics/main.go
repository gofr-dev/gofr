package main

import (
	"time"

	"gofr.dev/pkg/gofr"
)

// This example pushes metrics to an OTLP collector/backend, configured entirely
// through environment variables (see configs/.env) — the application code is
// identical to a pull-based Prometheus setup. The Prometheus /metrics endpoint
// stays available too; set METRICS_PORT=0 for push-only (serverless) mode.
//
// See README.md for backend recipes (OTel Collector, Datadog, Grafana Cloud,
// New Relic) and Google Managed Prometheus (examples/using-gcp-metrics).

const (
	ordersProcessed = "orders_processed"
	orderValue      = "order_value"
)

func main() {
	a := gofr.New()

	a.Metrics().NewCounter(ordersProcessed, "total number of processed orders")
	a.Metrics().NewHistogram(orderValue, "distribution of order values", 10, 50, 100, 500, 1000)

	a.POST("/order", OrderHandler)

	a.Run()
}

func OrderHandler(c *gofr.Context) (any, error) {
	c.Metrics().IncrementCounter(c, ordersProcessed)
	c.Metrics().RecordHistogram(c, orderValue, float64(time.Now().UnixNano()%1000))

	return "order processed", nil
}
