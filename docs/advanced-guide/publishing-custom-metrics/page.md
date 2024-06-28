# Publishing Custom Metrics

GoFr publishes some {% new-tab-link newtab=false title="default metrics" href="/docs/quick-start/observability" /%}.

GoFr can handle multiple different metrics concurrently, each uniquely identified by its name during initialization.
It supports the following {% new-tab-link title="metrics" href="https://opentelemetry.io/docs/specs/otel/metrics/" /%} types in Prometheus format:

1. `Counter`
2. `UpDownCounter`
3. `Histogram`
4. `Gauge`

If any custom metric is required, it can be created by using custom metrics as shown below:

## Usage

## 1. Counter Metrics

Counter is a {% new-tab-link title="synchronous Instrument" href="https://opentelemetry.io/docs/specs/otel/metrics/api/#synchronous-instrument-api" /%} which supports non-negative increments.

### Usage

```go
package main

import (
	"gofr.dev/pkg/gofr"
)

func main() {
	// initialise gofr object
	app := gofr.New()

	app.Metrics().NewCounter("transaction_success", "used to track the count of successful transactions")

	app.POST("/transaction", func(ctx *gofr.Context) (interface{}, error) {
		ctx.Metrics().IncrementCounter(ctx, "transaction_success")

		return "Transaction Successful", nil
	})

	app.Run()
}
```

## 2. UpDown Counter Metrics

`UpDownCounter` is a {% new-tab-link title="synchronous Instrument" href="https://opentelemetry.io/docs/specs/otel/metrics/api/#synchronous-instrument-api" /%} which supports increments and decrements.
Note: If the value is monotonically increasing, use Counter instead.

### Usage

```go
package main

import (
	"gofr.dev/pkg/gofr"
)

func main() {
	// initialise gofr object
	app := gofr.New()

	app.Metrics().NewUpDownCounter("total_credit_day_sale", "used to track the total credit sales in a day")

	app.POST("/sale", func(ctx *gofr.Context) (interface{}, error) {
		ctx.Metrics().DeltaUpDownCounter(ctx, "total_credit_day_sale", 1000)

		return "Sale Completed", nil
	})

	app.Run()
}
```

## 3. Histogram Metrics

Histogram is a {% new-tab-link title="synchronous Instrument" href="https://opentelemetry.io/docs/specs/otel/metrics/api/#synchronous-instrument-api" /%} which can be used to
report arbitrary values that are likely to be statistically meaningful. It is intended for statistics such as histograms, summaries, and percentile.

### Usage

```go
package main

import (
	"gofr.dev/pkg/gofr"
)

func main() {
	// initialise gofr object
	app := gofr.New()

	app.Metrics().NewHistogram("transaction_time", "used to track the time taken by a transaction",
		5, 10, 15, 20, 25, 35)

	app.POST("/transaction", func(ctx *gofr.Context) (interface{}, error) {
		transactionStartTime := time.Now()

		// transaction logic

		tranTime := time.Now().Sub(transactionStartTime).Milliseconds()

		ctx.Metrics().RecordHistogram(ctx, "transaction_time", float64(tranTime))

		return "Transaction Completed", nil
	})

	app.Run()
}
```

## 4. Gauge Metrics

Gauge is a {% new-tab-link title="synchronous Instrument" href="https://opentelemetry.io/docs/specs/otel/metrics/api/#synchronous-instrument-api" /%} which can be used to record non-additive value(s) when changes occur.

### Usage

```go
package main

import (
	"gofr.dev/pkg/gofr"
)

func main() {
	// initialise gofr object
	app := gofr.New()

	app.Metrics().NewGauge("product_stock", "used to track the number of products in stock")

	app.POST("/sale", func(ctx *gofr.Context) (interface{}, error) {
		ctx.Metrics().SetGauge("product_stock", 10)

		return "Sale Completed", nil
	})

	app.Run()
}
```

## Adding Labels to Custom Metrics

GoFr leverages metrics support by enabling labels. Labels are a key feature in metrics that allow you to categorize and filter metrics based on relevant information.

### Understanding Labels

Labels are key-value pairs attached to metrics. They provide additional context about the metric data.

Common examples of labels include:
- environment: (e.g., "production", "staging")
- service: (e.g., "api-gateway", "database")
- status: (e.g., "success", "failure")

By adding labels, you can create different time series for the same metric based on the label values.
This allows for more granular analysis and visualization in Grafana (or any other) dashboards.

### Additional Considerations

- Prefer to keep the number of labels manageable to avoid overwhelming complexity.
- Choose meaningful label names that clearly describe the data point.
- Ensure consistency in label naming conventions across your application.

By effectively using labels in GoFr, you can enrich your custom metrics and gain deeper insights into your application's performance and behavior.

### Usage:

Labels are added while populating the data for metrics, by passing them as arguments (comma separated key-value pairs)
in the GoFr's methods (namely: `IncreamentCounter`, `DeltaUpDownCounter`, `RecordHistogram`, `SetGauge`).

Example: `c.Metrics().IncrementCounter(c, "metric-name", "metric-value", "label-1", "value-1", "label-2", "value-2")`

```go
package main

import (
	"gofr.dev/pkg/gofr"
)

func main() {
	// Initialise gofr object
	a := gofr.New()

	// Add custom metrics
	a.Metrics().NewUpDownCounter("total_credit_day_sale", "used to track the total credit sales in a day")

	// Add all the routes
	a.POST("/sale", SaleHandler)
	a.POST("/return", ReturnHandler)

	// Run the application
	a.Run()
}

func SaleHandler(c *gofr.Context) (interface{}, error) {
	// logic to create sales

	c.Metrics().DeltaUpDownCounter(c, "total_credit_day_sale", 10, "sale_type", "credit", "product_type", "beverage") // Here "sale_type" & "product_type" are the labels and "credit" & "beverage" are the values

	return "Sale Successful", nil
}

func ReturnHandler(c *gofr.Context) (interface{}, error) {
	// logic to create a sales return

	c.Metrics().DeltaUpDownCounter(c, "total_credit_day_sale", -5, "sale_type", "credit_return", "product_type", "dairy")

	return "Return Successful", nil
}
```

**Good To Know**

```doc
While registering a metrics 2 key pieces of information of required:
- Name
- Description

When a registered metrics has to be used 3 key pieces of information are required:
- Name
- Value
- A set of key-value pairs called tags or labels.

A permutation of these key-value values provides the metric cardinality.
Lower the cardinality, faster the query performance and lower the monitoring resource utilisation.
```
