---
description: "Add custom OpenTelemetry spans to GoFr handlers for granular tracing. Wrap operations in named spans to surface latency in Jaeger, Tempo, or any OTLP backend."
nextjs:
  metadata:
    title: "Custom Spans in GoFr Tracing — OpenTelemetry Granularity"
    description: "Add custom OpenTelemetry spans to GoFr handlers for granular tracing. Wrap operations in named spans to surface latency in Jaeger, Tempo, or any OTLP backend."
---

# Custom Spans In Tracing

GoFr's built-in tracing provides valuable insights into application's behavior. However, sometimes we might need 
even more granular details about specific operations within your application. This is where `custom spans` can be used.

## How it helps?
By adding custom spans in traces to our requests, we can:

- **Gain granular insights:** Custom spans allows us to track specific operations or functions within your application, 
     providing detailed performance data.
- **Identify bottlenecks:** Analyzing custom spans helps to pinpoint areas of your code that may be causing 
      performance bottlenecks or inefficiencies.
- **Improve debugging:** Custom spans enhance the ability to debug issues by providing visibility into the execution 
      flow of an application.

## Usage

To add a custom trace to a request, GoFr context provides `Trace()` method, which takes the name of the span as an argument 
and returns a trace.Span. 

```go
import "gofr.dev/pkg/gofr"

func MyHandler(c *gofr.Context) (any, error) {
	span := c.Trace("my-custom-span")
	defer span.End()

	// Do some work here
	return nil, nil
}
```

In this example, **my-custom-span** is the name of the custom span that is added to the request.
The defer statement ensures that the span is closed even if an error occurs to ensure that the trace is properly recorded.

> ##### Check out the example of creating a custom span in GoFr: [Visit GitHub](https://github.com/gofr-dev/gofr/blob/main/examples/http-server/main.go#L58)

## Using the context from another goroutine

`Trace()` makes the new span the parent of everything that follows, by replacing the context held by `c`.
Because it changes `c`, call it only from the goroutine that runs the handler. Calling it from a goroutine
the handler started, while the handler keeps using `c`, is a data race.

From another goroutine, use `WithContext`. It returns a new `*gofr.Context` that carries the given context,
and leaves `c` unchanged. Its logs carry the trace ID of that context:

```go
import (
	"sync"

	"go.opentelemetry.io/otel"
)

func MyHandler(c *gofr.Context) (any, error) {
	var wg sync.WaitGroup

	wg.Go(func() {
		spanCtx, span := otel.Tracer("my-service").Start(c, "parallel-work")
		defer span.End()

		pc := c.WithContext(spanCtx)

		pc.Info("working in parallel") // logged with the parallel-work span's trace ID
		// pass pc (not c) to datasource calls made on this goroutine
	})

	// ... the handler keeps using c here ...

	wg.Wait()

	return nil, nil
}
```

The copy shares the request, the container and the response with `c`, so use it only while the request is still
being handled.

## Related production guides

- **Production Tracing**: [Configure OpenTelemetry exporters and samplers](/docs/guides/production-tracing) — wire your custom spans into a real OTLP collector.
- **Distributed Tracing**: [End-to-end traces across services](/docs/guides/distributed-tracing) — propagate trace context for inter-service spans.
