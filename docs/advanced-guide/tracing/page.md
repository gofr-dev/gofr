# Tracing
Tracing is a powerful tool for gaining insights into your application's behaviour, identifying bottlenecks, and improving
system performance. A trace is a tree of spans. It is a collective of observable signals showing the path of work
through a system. A trace on its own is distinguishable by a `TraceID`.

To know more about Tracing click [here](https://opentelemetry.io/docs/concepts/signals/#traces).

## Why it is important?

In complex distributed systems, understanding how requests flow through the system is crucial for troubleshooting performance
issues and identifying bottlenecks. Traditional logging approaches often fall short, providing limited visibility into
the intricate interactions between components.


## Automated Tracing in GoFr
GoFr makes it easy to use tracing by automatically adding traces to all requests and responses. GoFr uses
[OpenTelemetry](https://opentelemetry.io/docs/concepts/what-is-opentelemetry/), a popular tracing framework, to
automatically add traces to all requests and responses.

## Configs for enabling tracing

To enable tracing in your gofr application use the following configs:

```dotenv
TRACER_HOST=<tracer_host>
TRACER_PORT=<tracer_port>
```

> **NOTE:** If the value of `TRACER_PORT` is not provided, goFr uses port 9411 by default.

To run `zipkin` docker container locally , use the below docker command:
```console
docker run --name gofr-zipkin -p 2005:9411 -d openzipkin/zipkin:latest
```
## GoFr's Automatic Correlation ID Propagation:

When a request enters your GoFr application, GoFr automatically generates a correlation ID and adds it to the request headers.
This correlation ID is then propagated to all downstream requests. This means that you can track a request as it travels
through your distributed system by simply looking at the correlation ID in the request headers.
