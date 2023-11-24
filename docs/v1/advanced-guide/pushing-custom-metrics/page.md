# Adding Custom Metrics

Gofr supports the following [metric](https://prometheus.io/docs/concepts/metric_types/#counter) types in prometheus format:

1.  Counter Metrics
2.  Gauge Metrics
3.  Histogram Metrics
4.  Summary Metrics

GoFr by default pushes some of these metrics, which can be found [here](/docs/v1/references/metrics).

Gofr is capable of handling multiple counter, gauge, histogram, and summary metrics concurrently, each uniquely identified by its name during initialization.

## Guidelines for Custom Metrics Registration

1.  Custom metrics support a maximum of **4** labels.
2.  Ensure the same number of labels during metric initialisation.
3.  Labels have a maximum cardinality of **100**; exceeding this results in an error.
4.  Avoid duplicate metric names within a metric type to prevent errors.

## 1. Counter Metrics

Counters continuously increase over time or reset to zero on system restarts, tracking events like requests served or errors encountered.

### Usage

```go
package main

import (
	"gofr.dev/pkg/gofr"
	"net/http"
)

func main() {
	// initialise gofr object
	app := gofr.New()

	// register metrics
	_ = app.NewCounter("Counter_Name", "Help for Counter", "label1", "label2")

	app.GET("/greet", func(ctx *gofr.Context) (interface{}, error) {
		val := 2.0

		// increment metrics count by 1
		_ = ctx.Metric.IncCounter("Counter_Name", "label1", "label2")
		// Add value to metrics
		_ = ctx.Metric.AddCounter("Counter_Name", val, "label1", "label2")

		return "Hello World", nil
	})

	// Starts the server, it will listen on the default port 8000.
	// it can be over-ridden through configs
	app.Start()
}
```

## 2. Gauge Metrics

- A gauge is a metric that represents a single numerical value that can arbitrarily go up and down.
- Gauges are commonly used for measurements such as temperatures or current memory usage, as well as for counts that can increase and decrease, such as the number of concurrent requests.

### Usage

```go
package main

import (
	"gofr.dev/pkg/gofr"
	"net/http"
)

func main() {
	// initialise gofr object
	app := gofr.New()

	// `Gauge_Name` and `Help for Gauge` should be of type string, while `labels` is a variadic argument of type string.
	_ = app.NewGauge("Gauge_Name", "Help for Gauge", "label1", "label2")

	app.GET("/greet", func(ctx *gofr.Context) (interface{}, error) {
		val := 2.0

		err := ctx.Metric.SetGauge("Gauge_Name", val, "label1", "label2")
		if err != nil {
			// handle error
		}
		return "Hello World", nil
	})

	// Starts the server, it will listen on the default port 8000.
	// it can be over-ridden through configs
	app.Start()
}
```

## 3. Histogram Metrics

A histogram collects and counts observations, typically measurements like request durations or response sizes, into customizable buckets. It also calculates the sum of all observed values.

When you use a histogram with a base metric name of `<basename>`, it reveals multiple time series during data collection:

- Cumulative counters for the observation buckets are shown as `<basename>_bucket{le="<upper inclusive bound>"}`
- The overall sum of all observed values is displayed as `<basename>_sum`
- The count of observed events is exposed as `<basename>_count`, which is identical to `<basename>_bucket{le="+Inf"}`

### Usage

- Using `app.NewHistogram(name, help, bucket, labels) `, one can register a Histogram metric which can be used to push metrics in the method.
- `bucket` should be type of type `[]float64`, while `labels` should be of type string but passed as comma-separated values.

```go
package main

import (
	"gofr.dev/pkg/gofr"
	"net/http"
)

func main() {
	// initialise gofr object
	app := gofr.New()

	_ = app.NewHistogram("Histogram_Name", "Help for Histogram", []float64{.1,.2,.4,.8,1.6,6.4}, "label1", "label2")

	app.GET("/greet", func(ctx *gofr.Context) (interface{}, error) {
		// value must be of type float64
        val := 2.0

        err := ctx.Metric.ObserveHistogram("Histogram_Name", val, "label1", "label2")
        if err != nil {
        // handle error
        }

		return "Hello World", nil
	})

	// Starts the server, it will listen on the default port 8000.
	// it can be over-ridden through configs
	app.Start()
}
```

## 4. Summary Metrics

A Summary collects observations, typically metrics like request duration and response sizes, and also provides a total count of observations and a sum of all observed values.

When you use a Summary with a base metric name of `<basename>`, it exposes several time series during data collection:

- Streaming φ-quantiles (0 ≤ φ ≤ 1) of observed events, displayed as `<basename>{quantile="<φ>"}`
- The total sum of all observed values, shown as `<basename>_sum`
- The count of observed events, exposed as `<basename>_count`

### Usage

- Using `k.NewSummary(name , help , labels)`, one can register a Summary metric which can be used to push metrics in the method.
- `name` and `help` should be of type string, while `labels` should be of type string but passed as comma-separated values.

### Usage

```go
package main

import (
	"gofr.dev/pkg/gofr"
	"net/http"
)

func main() {
	// initialise gofr object
	app := gofr.New()

	err := app.NewSummary("Summary_Name", "Help for Histogram", "label1", "label2")

	app.GET("/greet", func(ctx *gofr.Context) (interface{}, error) {
		// value must be of type float64
        val := 2.0

        err := ctx.Metric.ObserveSummary("Summary_Name", val, "label1", "label2")
        if err != nil {
        // handle error
        }

		return "Hello World", nil
	})

	// Starts the server, it will listen on the default port 8000.
	// it can be over-ridden through configs
	app.Start()
}
```
