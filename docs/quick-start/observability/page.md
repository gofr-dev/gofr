# Observability

GoFr, by default, manages observability in different ways once the server starts:

## Logs

Logs offer real-time information, providing valuable insights and immediate visibility into the ongoing state and activities of the system.
It helps in identifying errors, debugging and troubleshooting, monitor performance, analyzing application usage, communications etc.

GoFr logger allows customizing the log level, which provides flexibility to adjust logs based on specific needs.

Logs are generated only for events equal to or above the specified log level; by default, GoFr logs at _INFO_ level.
Log Level can be changed by setting the environment variable `LOG_LEVEL` value to _WARN,DEBUG,ERROR,NOTICE or FATAL_.

When the GoFr server runs, it prints a log for reading configs, database connection, requests, database queries, missing configs, etc.
They contain information such as request's correlation ID, status codes, request time, etc.

The following table outlines the specific significance of each log level and provides guidelines on when to use them effectively:

{% table %}

- Levels
- Color
- Description
- When to use

---

- DEBUG 
- `Grey`
- extremely detailed or low-priority details for development.
- Variable values, loop steps, raw payloads, etc.

---

- INFO
- `Cyan`
- Standard operational events indicating system health.
- Server startup confirmation, successful requests, and routine health checks, etc.

---

- NOTICE
- `Yellow`
- Normal but distinct conditions requiring attention.
- Configuration updates, significant state changes, or system initialization events, etc.


---

- WARN 
- `Yellow`
- Potential issues handled gracefully without system failure.
- Deprecated API calls, connection retries, or approaching resource limits, etc.

---

- ERROR 
- `Red`
- Failure of specific requests or operations.
- Database connection failures, 4xx/5xx responses, or internal logic errors, etc.

---

- FATAL 
- `Red`
- Critical system failures forcing immediate shutdown.
- Missing required configuration or port binding failures preventing startup, etc.

{% /table %}

---

### DEBUG
This is the lowest priority level (Integer value: 1). It represents the most detailed information.

**Example Snippet-**
```Go
// Example: Checking a variable inside a loop
// Assuming i = 5 and item.Value = 42
logger.Debug("Processing item index:", i, "Value:", item.Value)
```
**Output-**
```Console
DEBU [14:05:01] [Processing item index: 5 Value: 42]
```
---
### INFO
Represents standard operational events (Integer value: 2). It is the default fallback level if an unknown level string is provided.

**Example Snippet-**
```Go
// Example: Server startup confirmation
logger.Info("Server started successfully on port 8000")
```
**Output-**
```Console
INFO [14:05:02] Server started successfully on port 8000
```

---

### NOTICE
A level higher than INFO but lower than WARN (Integer value: 3). It shares the same visual prominence as a Warning but implies a "normal" condition rather than a problem.

**Example Snippet-**
```Go
// Example: Configuration update
logger.Notice("Configuration hot-reload triggered by system admin")
```
**Output-**
```Console
NOTI [14:05:03] Configuration hot-reload triggered by system admin
```

---

### WARN
Indicates a potential issue that does not stop the application (Integer value: 4).

**Example Snippet-**
```Go
// Example: Retrying a connection
logger.Warn("Database connection timeout. Retrying in 2 seconds... (Attempt 1/3)")
```
**Output-**
```Console
WARN [14:05:04] Database connection timeout. Retrying in 2 seconds... (Attempt 1/3)
```

---

### ERROR
Indicates a failure event (Integer value: 5). Crucially, this level changes the output destination to the error stream.

**Example Snippet-**
```Go
// Example: Database query failure
logger.Error("Failed to fetch user profile: database connection refused")
```
**Output-**
```Console
ERRO [14:05:05] Failed to fetch user profile: database connection refused
```

---

### FATAL
The highest priority level (Integer value: 6). It represents a critical system failure.

**Example Snippet-**
```Go
// Example: Missing critical config
logger.Fatal("CRITICAL: DATABASE_PASSWORD environment variable is not set. Exiting.")
```
**Output-**
```Console
FATA [14:05:06] CRITICAL: DATABASE_PASSWORD environment variable is not set. Exiting.
```
---

{% figure src="/quick-start-logs.png" alt="Pretty Printed Logs" /%}

Logs are well-structured, they are of type JSON when exported to a file, such that they can be pushed to logging systems such as {% new-tab-link title="Loki" href="https://grafana.com/oss/loki/" /%}, Elasticsearch, etc.

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

For example: When running the application locally, we can access the /metrics endpoint on port 2121 from: {% new-tab-link title="http://localhost:2121/metrics" href="http://localhost:2121/metrics" /%}

GoFr also supports creating {% new-tab-link newtab=false title="custom metrics" href="/docs/advanced-guide/publishing-custom-metrics" /%}.

### Example Dashboard

These metrics can be easily consumed by monitoring systems like {% new-tab-link title="Prometheus" href="https://prometheus.io/" /%}
and visualized in dashboards using tools like {% new-tab-link title="Grafana" href="https://grafana.com/" /%}.

Here's a sample Grafana dashboard utilizing GoFr's metrics:

{% figure src="/metrics-dashboard.png" alt="Grafana Dashboard showing GoFr metrics including HTTP request rates, 
response times, etc." caption="Example monitoring dashboard using GoFr's built-in metrics" /%}


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
to the response headers. This correlation ID is then propagated to all downstream requests. This means that user can track
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
TRACER_RATIO=0.1

LOG_LEVEL=DEBUG
```

> [!NOTE]
> If the value of `TRACER_PORT` is not provided, GoFr uses port `9411` by default.

Open {% new-tab-link title="zipkin" href="http://localhost:2005/zipkin/" /%} and search by TraceID (correlationID) to see the trace.
{% figure src="/quick-start-trace.png" alt="Zipkin traces" /%}

#### 2. [Jaeger](https://www.jaegertracing.io/):

To see the traces, install Jaeger image using the following Docker command:

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
TRACER_RATIO=0.1
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
TRACER_RATIO=0.1
```



#### 4. [GoFr Tracer](https://tracer.gofr.dev/):

GoFr tracer is GoFr's own custom trace exporter as well as collector. Users can search a trace by its TraceID (correlationID)
in GoFr's own tracer service, available anywhere, anytime.

Add GoFr Tracer configs in `.env` file, your .env will be updated to
```dotenv
# ... no change in other env variables

# tracing configs
TRACE_EXPORTER=gofr
TRACER_RATIO=0.1
```

> [!NOTE]
> `TRACER_RATIO` refers to the proportion of traces that are exported through sampling. It ranges between 0 and 1. By default, this ratio is set to 1, meaning all traces are exported.
>
> Open {% new-tab-link title="gofr-tracer" href="https://tracer.gofr.dev/" /%} and search by TraceID (correlationID) to see the trace.
