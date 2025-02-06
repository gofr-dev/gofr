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
func MyHandler(c context.Context) error {
	span := c.Trace("my-custom-span")
	defer span.Close()

	// Do some work here
	return nil
}
```

In this example, **my-custom-span** is the name of the custom span that is added to the request.
The defer statement ensures that the span is closed even if an error occurs to ensure that the trace is properly recorded.

> ##### Check out the example of creating a custom span in GoFr: [Visit GitHub](https://github.com/gofr-dev/gofr/blob/main/examples/http-server/main.go#L58)
