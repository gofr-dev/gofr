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

  Logs are well-structured, they are of type JSON when exported to a file, such that they can be pushed to logging systems such as [Loki](https://grafana.com/oss/loki/), elastic search etc.


## Metrics

  GoFr gathers and pushes _essential metrics_ for different datastores(sql, redis), memory utilisation, request-response statistics etc.
  automatically to port: _2121_ on _/metrics_ endpoint in prometheus format.

  For example: When running application locally, you can access /metrics endpoint on port 2121 from: [http://localhost:2121/metrics](http://localhost:2121/metrics)

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
