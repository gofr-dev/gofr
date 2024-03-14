# Observability

Now that you have created your server, lets see how GoFr by default manages observability in different ways:

## Logs
  Logs offer real-time information, providing valuable insights and immediate visibility into the ongoing state and activities of the system.
  It helps in identifying errors, debugging and troubleshooting, monitor performance, analysing application usage, communications etc.

  GoFr logger has customizable log level which provides flexibility to adjust logs based on specific needs.

  Logs are generated only for events equal to or above the specified log level, by default GoFr logs at _INFO_ level.
  Log Level can be changed by setting the environment variable `LOG_LEVEL` value to _WARN,DEBUG,ERROR,NOTICE or FATAL_.

  When we run our server we see the following - logs for reading configs, database connection, requests, database queries, logs for missing configs etc.
  They contain information such as request's correlation ID, status codes, request time etc.

{% figure src="/quick-start-logs.png" alt="Pretty Printed Logs" /%}

  Logs are well-structured, they are of type JSON when exported to a file, such that they can be pushed to logging systems such as {% new-tab-link title="Loki" href="https://grafana.com/oss/loki/" /%}, elastic search etc.
  

## Metrics
Metrics enable performance monitoring by providing insights into response times, latency, throughput, and resource utilization.

They aid in tracking CPU, memory, and disk I/O consumption across services, facilitating capacity planning and scalability efforts.

Metrics play a pivotal role in fault detection and troubleshooting, offering visibility into system behavior.

They are instrumental in measuring and meeting service-level agreements (SLAs) to ensure expected performance and reliability.

GoFr by default publishes metrics automatically to port: _2121_ on _/metrics_ endpoint in prometheus format.

{% table %}

* Name
* Type
* Description
---
* app_go_numGC
* gauge
* Number of completed Garbage Collector cycles
---
* app_go_routines
* gauge
* Number of Go routines running
---
* app_go_sys
* gauge
* Number of total bytes of memory
---
* app_sys_memory_alloc
* gauge
* Number of bytes allocated for heap objects
---
* app_sys_total_alloc
* gauge
* Number of cumulative bytes allocated for heap objects
---
* app_http_response
* histogram
* Response time of http requests in seconds
---
* app_http_service_response
* histogram
* Response time of http service requests in seconds
---
* app_sql_open_connections
* gauge
* Number of open SQL connections
---
* app_sql_inUse_connections
* gauge
* Number of inUse SQL connections
---
* app_sql_stats
* histogram
* Response time of SQL queries in microseconds
---
* app_redis_stats
* histogram
* Response time of Redis commands in microseconds
---
* app_pubsub_publish_total_count
* counter
* Number of total publish operations
---
* app_pubsub_publish_success_count
* counter
* Number of successful publish operations
---
* app_pubsub_subscribe_total_count
* counter
* Number of total subscribe operations
---
* app_pubsub_subscribe_success_count
* counter
* Number of successful subscribe operations

{% /table %}

For example: When running application locally, you can access /metrics endpoint on port 2121 from: {% new-tab-link title="http://localhost:2121/metrics" href="http://localhost:2121/metrics" /%}

  GoFr also provides supports to create requirement specific metrics using {% new-tab-link title="custom metrics" href="/docs/advanced-guide/publishing-custom-metrics" /%}.
  
## Tracing
Tracing is a powerful tool for gaining insights into your application's behaviour, identifying bottlenecks, and improving
system performance. A trace is a tree of spans. It is a collective of observable signals showing the path of work
through a system. A trace on its own is distinguishable by a `TraceID`.

In complex distributed systems, understanding how requests flow through the system is crucial for troubleshooting performance
issues and identifying bottlenecks. Traditional logging approaches often fall short, providing limited visibility into
the intricate interactions between components.


To know more about Tracing click {% new-tab-link title="here" href="https://opentelemetry.io/docs/concepts/signals/#traces" /%}.


### Automated Tracing in GoFr
GoFr makes it easy to use tracing by automatically adding traces to all requests and responses. GoFr uses
{% new-tab-link title="OpenTelemetry" href="https://opentelemetry.io/docs/concepts/what-is-opentelemetry/" /%} , a popular tracing framework, to
automatically add traces to all requests and responses.

**Automatic Correlation ID Propagation:**

When a request enters your GoFr application, GoFr automatically generates a correlation ID X-Correlation-ID and adds it 
to the response headers. This correlation ID is then propagated to all downstream requests. This means that you can track
a request as it travels through your distributed system by simply looking at the correlation ID in the request headers.


### Configuration & Usage
To see the traces install zipkin image using the following docker command
```bash
  docker run --name gofr-zipkin -p 2005:9411 -d openzipkin/zipkin:latest
  ```

Add Tracer configs in `.env` file, your .env will be updated to

  ```bash
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
  TRACER_HOST=localhost
  TRACER_PORT=2005
  
  LOG_LEVEL=DEBUG
  ```

> **NOTE:** If the value of `TRACER_PORT` is not 
provided, gofr uses port `9411` by default.

Open {% new-tab-link title="zipkin" href="http://localhost:2005/zipkin/" /%} and search by TraceID (correlationID) to see the trace.
{% figure src="/quick-start-trace.png" alt="Pretty Printed Logs" /%}
