# GoFr Configuration Options

This document lists all the configuration options supported by the GoFr framework. The configurations are grouped by category for better organization.

## App

{% table %}

- Name
- Description
- Default Value

---

-  APP_NAME
-  Name of the application
-  gofr-app

---

-  APP_ENV
-  Name of the environment file to use (e.g., stage.env, prod.env, or local.env).

---

-  APP_VERSION
-  Application version
-  dev

---

-  LOG_LEVEL
-  Level of verbosity for application logs. Supported values are **DEBUG, INFO, NOTICE, WARN, ERROR, FATAL**
-  INFO

---

-  REMOTE_LOG_URL
-  URL to remotely change the log level

---

-  REMOTE_LOG_FETCH_INTERVAL
-  Time interval (in seconds) to check for remote log level updates
-  15

---

-  METRICS_PORT
-  Port on which the application exposes metrics
-  2121

---

-  HTTP_PORT
-  Port on which the HTTP server listens
-  8000

---

-  GRPC_PORT
-  Port on which the gRPC server listens
-  9000

---

-  TRACE_EXPORTER
-  Tracing exporter to use. Supported values: gofr, zipkin, jaeger, otlp.

---

-  TRACER_HOST
-  Hostname of the tracing collector. Required if TRACE_EXPORTER is set to zipkin or jaeger.
-  **DEPRECATED**

---

-  TRACER_PORT
-  Port of the tracing collector. Required if TRACE_EXPORTER is set to zipkin or jaeger.
-  9411
-  **DEPRECATED**

---

-  TRACER_URL
-  URL of the trace collector. Required if TRACE_EXPORTER is set to zipkin or jaeger.

---

-  TRACER_RATIO
-  Refers to the proportion of traces that are exported through sampling. It is optional configuration. By default, this ratio is set to 1.

---

-  TRACER_AUTH_KEY
-  Authorization header for trace exporter requests.
-  Supported for zipkin, jaeger.

---

-  CMD_LOGS_FILE
-  File to save the logs in case of a CMD application

---

-  SHUTDOWN_GRACE_PERIOD
-  Timeout duration for server shutdown process
-  30s

---

-  GOFR_TELEMETRY
-  Enable telemetry for GoFr framework usage
-  true

---

-  LOG_DISABLE_PROBES
-  Disable log probes for health checks
-  false

---

-  GRPC_ENABLE_REFLECTION
-  Enable gRPC server reflection
-  false


{% /table %}

## HTTP

{% table %}

- Name
- Description

---

-  REQUEST_TIMEOUT
-  Set the request timeouts (in seconds) for HTTP server.

---

- CERT_FILE
- Set the path to your PEM certificate file for the HTTPS server to establish a secure connection.

--- 

- KEY_FILE
- Set the path to your PEM key file for the HTTPS server to establish a secure connection.

{% /table %}


## Datasource

### SQL

{% table %}

- Name
- Description
- Default Value

---

-  DB_DIALECT
-  Database dialect. Supported values: mysql, postgres, supabase

---

-  DB_HOST
-  Hostname of the database server.

---

-  DB_PORT
-  Port of the database server.
-  3306

---

-  DB_USER
-  Username for the database.

---

-  DB_PASSWORD
-  Password for the database.

---

-  DB_NAME
-  Name of the database to use.

---

-  DB_MAX_IDLE_CONNECTION
-  Number of maximum idle connection.
-  2

---

-  DB_MAX_OPEN_CONNECTION
-  Number of maximum connections which can be used with database.
-  0 (unlimited)
---

-  DB_SSL_MODE
-  Currently supported only for PostgreSQL, with Default certificate file.
-  disable

---

- DB_REPLICA_HOSTS
- Comma-separated list of replica database hosts. Used for read replicas.
- None

---

- DB_REPLICA_PORTS
- Comma-separated list of replica database ports. Used for read replicas.
- None

---

- DB_REPLICA_USERS
- Comma-separated list of replica database users. Used for read replicas.
- None

---

- DB_REPLICA_PASSWORDS_
- Comma-separated list of replica database passwords. Used for read replicas.
- None

---

- DB_REPLICA_MAX_IDLE_CONNECTIONS
- Maximum idle connections allowed for a replica
- 50

---

- DB_REPLICA_MIN_IDLE_CONNECTIONS
- Minimum idle connections for a replica
- 10

---

- DB_REPLICA_DEFAULT_IDLE_CONNECTIONS
- Idle connections used if no primary setting is provided
- 10

---

- DB_REPLICA_MAX_OPEN_CONNECTIONS
- Maximum open connections allowed for a replica
- 200

---

- DB_REPLICA_MIN_OPEN_CONNECTIONS
- Minimum open connections for a replica
- 50

---

- DB_REPLICA_DEFAULT_OPEN_CONNECTIONS
- Open connections used if no primary setting is provided
- 100

---


- DB_CHARSET
- The character set for database connection
- utf8

---

- SUPABASE_CONNECTION_TYPE 
- Connection type to Supabase. Supported values: direct, session, transaction 
- direct

---

- SUPABASE_PROJECT_REF 
- Supabase project reference ID

---

- SUPABASE_REGION 
- Supabase region for pooled connections

---

- DB_URL 
- Full PostgreSQL connection string for Supabase (alternative to separate config parameters)

{% /table %}

### Redis

{% table %}

- Name
- Description
- Default Value

---

-  REDIS_HOST
-  Hostname of the Redis server.
-  localhost

---

-  REDIS_PORT
-  Port of the Redis server.
-  6379

---

- REDIS_USER
- Username for the Redis server (optional).
-  ""

---

- REDIS_PASSWORD
- Password for the Redis server (optional).
-  ""

---

- REDIS_DB
- Database number to use for the Redis server.
-  0

---

- REDIS_TLS_ENABLED
- Enable TLS for Redis connections.
-  false

---

- REDIS_TLS_CA_CERT
- Path to the TLS CA certificate file for Redis (or PEM-encoded string).
-  ""

---

- REDIS_TLS_CERT
- Path to the TLS certificate file for Redis (or PEM-encoded string).
-  ""

---

- REDIS_TLS_KEY
- Path to the TLS key file for Redis (or PEM-encoded string).
-  ""

{% /table %}

**Redis PubSub Configuration:**

{% table %}

- Name
- Description
- Default Value

---

- REDIS_PUBSUB_MODE
- Operation mode: `pubsub` or `streams`.
- pubsub

---

- REDIS_STREAMS_CONSUMER_GROUP
- Consumer group name (required for streams mode).
-  ""

---

- REDIS_STREAMS_CONSUMER_NAME
- Unique consumer name (optional, auto-generated if empty).
-  ""

---

- REDIS_STREAMS_BLOCK_TIMEOUT
- Blocking duration for reading new messages.
-  5s

---

- REDIS_STREAMS_MAXLEN
- Maximum length of the stream (approximate).
-  0 (unlimited)

{% /table %}

### Pub/Sub

{% table %}


- Name
- Description
- Default Value

---

-  PUBSUB_BACKEND
-  Pub/Sub message broker backend
-  kafka, google, mqtt, nats, redis

{% /table %}

**Kafka**

{% table %}


- Name
- Description
- Default Value

---

-  PUBSUB_BROKER
-  Comma-separated list of broker addresses
-  localhost:9092

---

-  PARTITION_SIZE
-  Size of each message partition (in bytes)
-  0

---

-  PUBSUB_OFFSET
-  Offset to start consuming messages from. -1 for earliest, 0 for latest.
-  -1

---

- KAFKA_BATCH_SIZE
- Number of messages to batch before sending to Kafka
- 1

---

- KAFKA_BATCH_BYTES
- Number of bytes to batch before sending to Kafka
- 1048576

---

- KAFKA_BATCH_TIMEOUT
- Time to wait before sending a batch to Kafka
- 100ms

---

-  CONSUMER_ID
-  Unique identifier for this consumer
-  gofr-consumer

---

---

- KAFKA_SECURITY_PROTOCOL
- Security protocol used to communicate with Kafka (e.g., PLAINTEXT, SSL, SASL_PLAINTEXT, SASL_SSL)
- PLAINTEXT

---


- KAFKA_SASL_MECHANISM
- SASL mechanism for authentication (e.g. PLAIN, SCRAM-SHA-256, SCRAM-SHA-512)
- None

---

- KAFKA_SASL_USERNAME
- Username for SASL authentication
- None

---

- KAFKA_SASL_PASSWORD
- Password for SASL authentication
- None

---

- KAFKA_TLS_CERT_FILE
- Path to the TLS certificate file
- None

---

- KAFKA_TLS_KEY_FILE
- Path to the TLS key file
- None

---

- KAFKA_TLS_CA_CERT_FILE
- Path to the TLS CA certificate file
- None

---

- KAFKA_TLS_INSECURE_SKIP_VERIFY
- Skip TLS certificate verification
- false

{% /table %}

**Google**

{% table %}


- Name
- Description

---

-  GOOGLE_PROJECT_ID
-  ID of the Google Cloud project. Required for Google Pub/Sub.

---

-  GOOGLE_SUBSCRIPTION_NAME
-  Name of the Google Pub/Sub subscription. Required for Google Pub/Sub.

{% /table %}

**Redis**

{% table %}

- Name
- Description
- Default Value

---

-  REDIS_PUBSUB_BUFFER_SIZE
-  Message channel buffer size for Redis Pub/Sub (channels/streams)
-  100

---

-  REDIS_PUBSUB_QUERY_TIMEOUT
-  Default timeout for Redis Pub/Sub Query operations
-  5s

---

-  REDIS_PUBSUB_QUERY_LIMIT
-  Default message limit for Redis Pub/Sub Query operations
-  10

---

-  REDIS_ADDR
-  Redis server address (host:port)
-  localhost:6379

---

-  REDIS_PASSWORD
-  Password for Redis authentication
-  None

---

-  REDIS_DB
-  Database number to use (0-15)
-  0

---

-  REDIS_MAX_RETRIES
-  Maximum number of retries for failed commands
-  3

---

-  REDIS_DIAL_TIMEOUT
-  Timeout for establishing connections
-  5s

---

-  REDIS_READ_TIMEOUT
-  Timeout for socket reads
-  3s

---

-  REDIS_WRITE_TIMEOUT
-  Timeout for socket writes
-  3s

---

-  REDIS_POOL_SIZE
-  Maximum number of socket connections in the pool
-  10

---

-  REDIS_MIN_IDLE_CONNS
-  Minimum number of idle connections
-  5

---

-  REDIS_MAX_IDLE_CONNS
-  Maximum number of idle connections
-  10

---

-  REDIS_CONN_MAX_IDLE_TIME
-  Maximum amount of time a connection may be idle
-  5m

---

-  REDIS_CONN_MAX_LIFETIME
-  Maximum amount of time a connection may be reused
-  30m

---

-  REDIS_TLS_CERT_FILE
-  Path to the TLS certificate file
-  None

---

-  REDIS_TLS_KEY_FILE
-  Path to the TLS key file
-  None

---

-  REDIS_TLS_CA_CERT_FILE
-  Path to the TLS CA certificate file
-  None

---

-  REDIS_TLS_INSECURE_SKIP_VERIFY
-  Skip TLS certificate verification
-  false

{% /table %}

**MQTT**

{% table %}


- Name
- Description
- Default Value

---

-  MQTT_PORT
-  Port of the MQTT broker
-  1883

---

-  MQTT_MESSAGE_ORDER
-  Enable guaranteed message order
-  false

---

-  MQTT_PROTOCOL
-  Communication protocol. Supported values: tcp, ssl.
-  tcp

---

-  MQTT_HOST
-  Hostname of the MQTT broker
-  localhost

---

-  MQTT_USER
-  Username for the MQTT broker

---

-  MQTT_PASSWORD
-  Password for the MQTT broker

---

-  MQTT_CLIENT_ID_SUFFIX
-  Suffix appended to the client ID

---

-  MQTT_QOS
-  Quality of Service Level

---

-  MQTT_KEEP_ALIVE
-  Sends regular messages to check the link is active. May not work as expected if handling func is blocking execution

- MQTT_RETRIEVE_RETAINED
- Retrieve retained messages on subscription

{% /table %}

**NATS JetStream**

{% table %}

- Name
- Description
- Default Value

---

-  NATS_SERVER
-  URL of the NATS server
-  nats://localhost:4222

---

-  NATS_CREDS_FILE
-  File containing the NATS credentials
- creds.json

{% /table %}

