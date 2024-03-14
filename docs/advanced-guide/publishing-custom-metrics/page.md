# Publishing Custom Metrics

For GoFr default metrics refer: {% new-tab-link title="observability" href="/docs/quick-start/observability" /%}.

GoFr can handle multiple different metrics concurrently, each uniquely identified by its name during initialization.
It supports the following {% new-tab-link title="metrics" href="https://opentelemetry.io/docs/specs/otel/metrics/" /%} types in prometheus format:

1. Counter
2. UpDownCounter
3. Histogram
4. Gauge

If any metric other than defaults provided, you can create them using custom metrics as shown below.


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

UpDownCounter is a {% new-tab-link title="synchronous Instrument" href="https://opentelemetry.io/docs/specs/otel/metrics/api/#synchronous-instrument-api" /%} which supports increments and decrements.
Note: if the value is monotonically increasing, use Counter instead.

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
		ctx.Metrics().IncrementCounter(ctx, "total_credit_day_sale")

		return "Sale Completed", nil
	})
	
	app.Run()
}
```

## 3. Histogram Metrics

Histogram is a {% new-tab-link title="synchronous Instrument" href="https://opentelemetry.io/docs/specs/otel/metrics/api/#synchronous-instrument-api" /%} which can be used to report arbitrary values that are likely to be statistically meaningful. It is intended for statistics such as histograms, summaries, and percentile.

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

		tranTime := time.Now().Sub(transactionStartTime).Microseconds()

		ctx.Metrics().RecordHistogram(ctx, "transaction_time", float64(tranTime))

		return "Transaction Completed", nil
	})

	app.Run()
}
```

## 4. Gauge Metrics

Gauge is a {% new-tab-link title="synchronous Instrument" href="https://opentelemetry.io/docs/specs/otel/metrics/api/#synchronous-instrument-api" /%} which can be used to record non-additive value(s)  when changes occur.
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

**Good To Know**
```doc
While registering a metrics two key pieces of information of required
- Name
- Description
   
When a registered Metrics has to be used 3 key pieces of information are required:
- Metrics name
- Value
- A set of key-value pairs called tags or labels.

A permutation of these key-value values provides the metric cardinality.
Lower the cardinality, faster the query performance and lower the monitoring resource utilisation.
```