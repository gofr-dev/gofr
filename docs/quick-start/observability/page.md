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

  GoFr gathers and pushes some _essential metrics_ for different datastores(sql, redis etc.), pubsubs, memory utilisation, request-response statistics etc.
  automatically to port [http://localhost:2121/metrics](http://localhost:2121/metrics) in prometheus format.

{% table %}

* Name
* Type
* Description
* Example
---
* app_go_numGC
* gauge
* Number of completed Garbage Collector cycles
* app_go_numGC{otel_scope_name="test-service",otel_scope_version="dev"} 2
---
* app_go_routines
* gauge
* Number of Go routines running
* app_go_routines{otel_scope_name="test-service",otel_scope_version="dev"} 10
---
* app_go_sys
* gauge
* Number of total bytes of memory
* app_go_sys{otel_scope_name="test-service",otel_scope_version="dev"} 1.3929488e+07
---
* app_sys_memory_alloc
* gauge
* Number of bytes allocated for heap objects
* app_sys_memory_alloc{otel_scope_name="test-service",otel_scope_version="dev"} 5.438312e+06
---
* app_sys_total_alloc
* gauge
* Number of cumulative bytes allocated for heap objects
* app_sys_total_alloc{otel_scope_name="test-service",otel_scope_version="dev"} 1.1268224e+07
---
* app_http_response
* histogram
* Response time of http requests in seconds
* app_http_response_bucket{method="GET",otel_scope_name="test-service",otel_scope_version="dev",path="/customer",status="200",le="0.005"} 1
---
* app_http_service_response
* histogram
* Response time of http service requests in seconds
* app_http_service_response_bucket{method="GET",otel_scope_name="test-service",otel_scope_version="dev",path="https://catfact.ninja",status="200",le="1"} 1
---
* app_sql_open_connections
* gauge
* Number of open SQL connections
* app_sql_open_connections{otel_scope_name="test-service",otel_scope_version="dev"} 1
---
* app_sql_inUse_connections
* gauge
* Number of inUse SQL connections
* app_sql_inUse_connections{otel_scope_name="test-service",otel_scope_version="dev"} 0
---
* app_sql_stats
* histogram
* Observes the response time for SQL queries
* app_sql_stats_bucket{otel_scope_name="test-service",otel_scope_version="dev",type="select",le="0.001"} 1
---
* app_redis_stats
* histogram
* Observes the response time for Redis commands
* app_redis_stats_bucket{otel_scope_name="test-service",otel_scope_version="dev",type="get",le="0.005"} 1

{% /table %}

  GoFr also provides supports to create requirement specific metrics using [custom metrics](/docs/advanced-guide/publishing-custom-metrics).


## Tracing

  GoFr adds traces by default for all the request and response,which allows you to export it to zipkin by adding the configs.
  It allows to monitor the request going through different parts of application like database, handler etc.

  To see the traces install zipkin image using the following docker command

  ### Setup

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
