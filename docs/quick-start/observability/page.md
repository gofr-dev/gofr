# Observability

GoFr by default manages observability in different ways once the server starts:

## Logs

Logs offer real-time information, providing valuable insights and immediate visibility into the ongoing state and activities of the system.
It helps in identifying errors, debugging and troubleshooting, monitor performance, analyzing application usage, communications etc.

GoFr logger allows to customize log level which provides flexibility to adjust logs based on specific needs.

Logs are generated only for events equal to or above the specified log level, by default GoFr logs at _INFO_ level.
Log Level can be changed by setting the environment variable `LOG_LEVEL` value to _WARN,DEBUG,ERROR,NOTICE or FATAL_.

When GoFr server runs, it prints log for reading configs, database connection, requests, database queries, missing configs etc.
They contain information such as request's correlation ID, status codes, request time etc.

{% figure src="/quick-start-logs.png" alt="Pretty Printed Logs" /%}

Logs are well-structured, they are of type JSON when exported to a file, such that they can be pushed to logging systems such as {% new-tab-link title="Loki" href="https://grafana.com/oss/loki/" /%}, elastic search etc.

## Metrics

Metrics enable performance monitoring by providing insights into response times, latency, throughput, resource utilization, tracking CPU, memory, and disk I/O consumption across services, facilitating capacity planning and scalability efforts.

Metrics play a pivotal role in fault detection and troubleshooting, offering visibility into system behavior.

They are instrumental in measuring and meeting service-level agreements (SLAs) to ensure expected performance and reliability.

GoFr publishes metrics to port: _2121_ on _/metrics_ endpoint in Prometheus format.

{% table %}

- Name
- Type
- Description

---

- app_go_numGC
- gauge
- Number of completed Garbage Collector cycles

---

- app_go_routines
- gauge
- Number of Go routines running

---

- app_go_sys
- gauge
- Number of total bytes of memory

---

- app_sys_memory_alloc
- gauge
- Number of bytes allocated for heap objects

---

- app_sys_total_alloc
- gauge
- Number of cumulative bytes allocated for heap objects

---

- app_info
- gauge
- Number of instances running with info of app and framework

---

- app_http_response
- histogram
- Response time of HTTP requests in seconds

---

- app_http_service_response
- histogram
- Response time of HTTP service requests in seconds

---

- app_sql_open_connections
- gauge
- Number of open SQL connections

---

- app_sql_inUse_connections
- gauge
- Number of inUse SQL connections

---

- app_sql_stats
- histogram
- Response time of SQL queries in milliseconds

---

- app_redis_stats
- histogram
- Response time of Redis commands in milliseconds

---

- app_pubsub_publish_total_count
- counter
- Number of total publish operations

---

- app_pubsub_publish_success_count
- counter
- Number of successful publish operations

---

- app_pubsub_subscribe_total_count
- counter
- Number of total subscribe operations

---

- app_pubsub_subscribe_success_count
- counter
- Number of successful subscribe operations

{% /table %}

For example: When running application locally, you can access /metrics endpoint on port 2121 from: {% new-tab-link title="http://localhost:2121/metrics" href="http://localhost:2121/metrics" /%}

GoFr also supports creating {% new-tab-link newtab=false title="custom metrics" href="/docs/advanced-guide/publishing-custom-metrics" /%}.

## Tracing

{% new-tab-link title="Tracing" href="https://opentelemetry.io/docs/concepts/signals/#traces" /%} is a powerful tool for gaining insights into your application's behavior, identifying bottlenecks, and improving
system performance. A trace is a tree of spans. It is a collective of observable signals showing the path of work
through a system. A trace on its own is distinguishable by a `TraceID`.

In complex distributed systems, understanding how requests flow through the system is crucial for troubleshooting performance
issues and identifying bottlenecks. Traditional logging approaches often fall short, providing limited visibility into
the intricate interactions between components.



### Automated Tracing in GoFr

GoFr automatically exports traces for all requests and responses. GoFr uses
{% new-tab-link title="OpenTelemetry" href="https://opentelemetry.io/docs/concepts/what-is-opentelemetry/" /%} , a popular tracing framework, to
automatically add traces to all requests and responses.

**Automatic Correlation ID Propagation:**

When a request enters your GoFr application, GoFr automatically generates a correlation-ID `X-Correlation-ID` and adds it
to the response headers. This correlation ID is then propagated to all downstream requests. This means that you can track
a request as it travels through your distributed system by simply looking at the correlation ID in the request headers.

### Configuration & Usage:

GoFr has support for following trace-exporters:
#### 1. [Zipkin](https://zipkin.io/): 

To see the traces install zipkin image using the following Docker command:

```bash
  docker run --name gofr-zipkin -p 2005:9411 -d openzipkin/zipkin:latest
```

Add Tracer configs in `.env` file, your .env will be updated to

```dotenv
APP_NAME=test-service
HTTP_PORT=9000

REDIS_HOST=localhost
REDIS_PORT=6379

DB_HOST=localhost
DB_USER=root
DB_PASSWORD=root123
DB_NAME=test_db
DB_PORT=3306

# tracing configs
TRACE_EXPORTER=zipkin
TRACER_URL=http://localhost:2005/api/v2/spans

LOG_LEVEL=DEBUG
```

> **NOTE:** If the value of `TRACER_PORT` is not
> provided, GoFr uses  port `9411` by default.

Open {% new-tab-link title="zipkin" href="http://localhost:2005/zipkin/" /%} and search by TraceID (correlationID) to see the trace.
{% figure src="/quick-start-trace.png" alt="Zipkin traces" /%}

#### 2. [Jaeger](https://www.jaegertracing.io/):

To see the traces install jaeger image using the following Docker command:

```bash
docker run -d --name jaeger \
  -e COLLECTOR_OTLP_ENABLED=true \
  -p 16686:16686 \
  -p 14317:4317 \
  -p 14318:4318 \
  jaegertracing/all-in-one:1.41
```

Add Jaeger Tracer configs in `.env` file, your .env will be updated to
```dotenv
# ... no change in other env variables

# tracing configs
TRACE_EXPORTER=jaeger
TRACER_URL=localhost:14317
```

Open {% new-tab-link title="jaeger" href="http://localhost:16686/trace/" /%} and search by TraceID (correlationID) to see the trace.
{% figure src="/jaeger-traces.png" alt="Jaeger traces" /%}

#### 3. [OpenTelemetry Protocol](https://opentelemetry.io/docs/specs/otlp/):

The OpenTelemetry Protocol (OTLP)  underlying gRPC is one of general-purpose telemetry data delivery protocol designed in the scope of the OpenTelemetry project.

Add OTLP configs in `.env` file, your .env will be updated to
```dotenv
# ... no change in other env variables

# tracing configs 
TRACE_EXPORTER=otlp
TRACER_URL=localhost:4317
```



#### 4. [GoFr Tracer](https://tracer.gofr.dev/):

GoFr tracer is GoFr's own custom trace exporter as well as collector. You can search a trace by its TraceID (correlationID)
in GoFr's own tracer service available anywhere, anytime.

Add GoFr Tracer configs in `.env` file, your .env will be updated to
```dotenv
# ... no change in other env variables

# tracing configs
TRACE_EXPORTER=gofr
```

Open {% new-tab-link title="gofr-tracer" href="https://tracer.gofr.dev/" /%} and search by TraceID (correlationID) to see the trace.
