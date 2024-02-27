# Custom Spans In Tracing

GoFr's built-in tracing provides valuable insights into your application's behavior. However, sometimes you might need 
even more granular details about specific operations within your application. This is where `custom spans` come in.

## How it helps?
By adding custom spans in traces to your requests, you can:

- **Gain granular insights:** Custom spans allow you to track specific operations or functions within your application, 
     providing detailed performance data.
- **Identify bottlenecks:** By analyzing custom spans, you can pinpoint areas of your code that may be causing 
      performance bottlenecks or inefficiencies.
- **Improve debugging:** Custom spans enhance your ability to debug issues by providing visibility into the execution 
      flow of your application.

## Usage

To add a custom trace to a request, you can use the `Trace()` method of GoFr context, which takes the name of the span as an argument 
and returns a trace.Span. 

```go
func MyHandler(c context.Context) error {
    span := c.Trace("my-custom-span")
    defer span.Close()
    
    // Do some work here
    return nil
}
```

In this example, **my-custom-span** is the name of the custom span that you want to add to the request.
The defer statement ensures that the span is closed even if an error occurs to ensure that the trace is properly recorded.



