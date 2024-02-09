# Observability

Now that you have created your server, lets see how GoFr by default manages observability in different ways:

## Logs

  When we run our server we see the following - logs for reading configs, database connection, requests, database queries, logs for missing configs etc.
  They contain information such as request's correlation ID, status codes, request time etc.

  Logs are generated only for events equal to or above the specified log level, by default GoFr logs at INFO level.

  Log Level can be changed by setting the environment variable `LOG_LEVEL` value to _WARN,DEBUG,ERROR or FATAL_.

  ```bash
    DEBU [16:32:47] Container is being created
    DEBU [16:32:47] ping                             REDIS  25488µs ping
    INFO [16:32:47] connected to redis at localhost:6379
    INFO [16:32:47] connected to 'test_db' database at localhost:3306
    INFO [16:32:47] Starting server on port: 9000
    INFO [16:32:47] Starting metrics server on port: 2121
    INFO [16:32:52] a9ce6af942307b323e89ab5368ebe784 200    14210µs GET /customer 
  ```

  Logs are well-structured, they are of type JSON when exported to a file, such that they can be pushed to logging systems such as [Loki](https://grafana.com/oss/loki/), elastic search etc.

## Metrics

  GoFr gathers and pushes [essential metrics](/docs/v1/references/metrics) for different datastores(sql, redis etc), pubsubs, memory utilisation, request-response statistics etc automatically to port [http://localhost:2121/metrics](http://localhost:2121/metrics) in prometheus format.

  ```bash
  # TYPE go_gc_duration_seconds summary
  go_gc_duration_seconds{quantile="0"} 4.275e-05
  go_gc_duration_seconds{quantile="0.25"} 4.275e-05
  go_gc_duration_seconds{quantile="0.5"} 6.8542e-05
  # TYPE app_go_numGC gauge
  app_go_numGC{otel_scope_name="test-service",otel_scope_version="dev"} 0
  # TYPE app_go_routines gauge
  app_go_routines{otel_scope_name="test-service",otel_scope_version="dev"} 10
  # TYPE app_go_sys gauge
  app_go_sys{otel_scope_name="test-service",otel_scope_version="dev"} 1.274984e+07
  # TYPE app_http_response histogram
  app_http_response_bucket{method="GET",otel_scope_name="test-service",otel_scope_version="dev",path="/customer",status="200",le="0.001"} 0
  app_http_response_bucket{method="GET",otel_scope_name="test-service",otel_scope_version="dev",path="/customer",status="200",le="0.003"} 0
  app_http_response_bucket{method="GET",otel_scope_name="test-service",otel_scope_version="dev",path="/customer",status="200",le="0.005"} 1
  ```

## Tracing

  GoFr adds traces by default and samples them for all the request and response, allows you to enable it by adding the configs.
  It allows to monitor the request going through different parts of our app like database, handler etc.

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

  ![Zipkin Trace Image](https://drive.google.com/file/d/1WzaKfrcPJD_NLSrXfCxlwuZqhyjQ8tNw/preview)
