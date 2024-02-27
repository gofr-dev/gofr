# Observability

Now that you have created your server, lets see how GoFr by default manages observability in different ways:

## Logs
  When we run our server we see the following - logs for reading configs, database connection, requests, database queries, logs for missing configs etc.
  They contain information such as request's correlation ID, status codes, request time etc.

  Logs are generated only for events equal to or above the specified log level, by default GoFr logs at _INFO_ level.

  Log Level can be changed by setting the environment variable `LOG_LEVEL` value to _WARN,DEBUG,ERROR,NOTICE or FATAL_.

  {% figure src="/quick-start-logs.png" alt="Pretty Printed Logs" /%}

  Logs are well-structured, they are of type JSON when exported to a file, such that they can be pushed to logging systems such as [Loki](https://grafana.com/oss/loki/), elastic search etc.

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
* Observes the response time for SQL queries
---
* app_redis_stats
* histogram
* Observes the response time for Redis commands

{% /table %}

For example: When running application locally, you can access /metrics endpoint on port 2121 from: [http://localhost:2121/metrics](http://localhost:2121/metrics)

  GoFr also provides supports to create requirement specific metrics using [custom metrics](/docs/advanced-guide/publishing-custom-metrics).


## Tracing

  GoFr adds traces by default for all the request and response,which allows you to export it to zipkin by adding the configs.
  It allows to monitor the request going through different parts of application like database, handler etc.

  To see the traces install zipkin image using the following docker command

  ```bash
  docker run --name gofr-zipkin -p 2005:9411 -d openzipkin/zipkin:latest
  ```

  ### Configuration & Usage

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
  
  TRACER_HOST=localhost
  TRACER_PORT=2005
  
  LOG_LEVEL=DEBUG
  ```

  Open [zipkin](http://localhost:2005/zipkin/) and search by TraceID (correlationID) to see the trace.

{% figure src="/quick-start-trace.png" alt="Pretty Printed Logs" /%}
