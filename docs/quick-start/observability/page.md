# Observability

Now that you have created your server, lets see how GoFr by default manages observability in different ways :

- ## Logs

  When we run our server we see the following - logs for reading configs, database connection, requests, database queries, logs for missing configs etc.
  They contain information such as request's correlation ID, status codes, request time etc.

  Logs are generated only for events equal to or above the specified log level, by default GoFr logs at INFO level.

  Log Level can be changed by setting the environment variable `LOG_LEVEL` value to _WARN,DEBUG,ERROR or FATAL_.

  ```bash
    INFO [15:09:17]  Loaded config from file:  ./configs/.env
                    (Memory: 6177016 GoRoutines: 2)
    WARN [15:09:17]  APP_VERSION is not set. 'dev' will be used in logs
                    (Memory: 6181960 GoRoutines: 2)
    INFO [15:09:17]  Redis connected. HostName: localhost, Port: 6379
                    (Memory: 6205704 GoRoutines: 3)
    INFO [15:09:17]  DB connected, HostName: localhost, Port: 3306, Database: test_db
                    (Memory: 6257992 GoRoutines: 5)
    INFO [15:09:17]  Starting metrics server at :2121
                    (Memory: 6354824 GoRoutines: 5)
    INFO [15:09:17]  GET /greet HEAD /greet POST /customer/{name} GET /customer HEAD /customer GET /.well-known/health-check HEAD /.well-known/health-check GET /.well-known/heartbeat HEAD /.well-known/heartbeat
                    (Memory: 6360552 GoRoutines: 6)
    INFO [15:09:17]  starting http server at :9000
                    (Memory: 6364856 GoRoutines: 7)
    DEBU [15:24:03] sql [SELECT * FROM customers] - 14.80ms
                    (Memory: 8530176 GoRoutines: 11)
    INFO [15:09:23]  GET /customer - 42.81ms (StatusCode: 200)
    CorrelationId: 3587c568770811ee8ec98230a0894a34 (Memory: 6524984 GoRoutines: 9)
  ```

  Logs are well-structured, they are of type JSON when exported to a file, such that they can be pushed to logging systems such as [Loki](https://grafana.com/oss/loki/), elastic search etc.

- ## Metrics

  GoFr gathers and pushes [essential metrics](/docs/v1/references/metrics) for different datastores(sql, redis etc), pubsubs, memory utilisation, request-response statistics etc automatically to port [http://localhost:2121/metrics](http://localhost:2121/metrics) in prometheus format.

  ```bash
  # TYPE go_gc_duration_seconds summary
  go_gc_duration_seconds{quantile="0"} 0.000159083
  go_gc_duration_seconds{quantile="0.25"} 0.000172459
  go_gc_duration_seconds{quantile="0.5"} 0.001140416
  # TYPE zs_http_response histogram
  zs_http_response_bucket{method="GET",path="/customer",status="200",le="0.001"} 0
  zs_http_response_bucket{method="GET",path="/customer",status="200",le="0.003"} 0
  # TYPE zs_sql_stats histogram
  zs_sql_stats_bucket{database="test_db",host="localhost",type="SELECT",le="0.4"} 1

  ```

- ## Tracing

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
  DB_DIALECT=mysql

  TRACER_EXPORTER=zipkin
  TRACER_URL=http://localhost:2005
  TRACER_ALWAYS_SAMPLE=true
  ```

  Open [zipkin](http://localhost:2005/zipkin/) and search by TraceID (correlationID) to see the trace.

  ![Zipkin Trace Image](https://drive.google.com/file/d/1WzaKfrcPJD_NLSrXfCxlwuZqhyjQ8tNw/preview)
