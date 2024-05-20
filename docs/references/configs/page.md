
# Gofr Configuration Options

This document lists all the configuration options supported by the Gofr framework. The configurations are grouped by category for better organization.

## App Configs

| Config | Description | Default Value |
|---|---|---|
| APP_NAME | Name of the application | gofr-app |
| APP_ENV | Name of the environment file to use (e.g., stage.env, prod.env, or local.env). | |
| APP_VERSION | Application version | dev |
| LOG_LEVEL | Level of verbosity for application logs. Supported values are **DEBUG, INFO, WARN, ERROR, NOTICE, FATAL** | INFO |
| REMOTE_LOG_URL | URL to remotely change the log level | |
| REMOTE_LOG_FETCH_INTERVAL | Time interval (in seconds) to check for remote log level updates | 15 |
| METRICS_PORT | Port on which the application exposes metrics | 2121 |
| HTTP_PORT | Port on which the HTTP server listens | 8000 |
| GRPC_PORT | Port on which the gRPC server listens | 9000 |
| TRACE_EXPORTER | Tracing exporter to use. Supported values: gofr, zipkin, jaeger. | gofr |
| TRACER_HOST | Hostname of the tracing collector. Required if TRACE_EXPORTER is set to zipkin or jaeger. | |
| TRACER_PORT | Port of the tracing collector. Required if TRACE_EXPORTER is set to zipkin or jaeger. | 9411 |

## Datasource Configs

| Config | Description | Supported Values (if applicable) |
|---|---|---|
| PUBSUB_BACKEND | Pub/Sub message broker backend | kafka, google, mqtt |

**For Kafka:**

| Config | Description | Default Value |
|---|---|---|
| PUBSUB_BROKER | Comma-separated list of broker addresses | localhost:9092 |
| PARTITION_SIZE | Size of each message partition (in bytes) | 0 |
| PUBSUB_OFFSET | Offset to start consuming messages from. -1 for earliest, 0 for latest. | -1 |
| CONSUMER_ID | Unique identifier for this consumer | gofr-consumer |

**For Google:**

| Config | Description | |
|---|---|---|
| GOOGLE_PROJECT_ID | ID of the Google Cloud project. Required for Google Pub/Sub. | |
| GOOGLE_SUBSCRIPTION_NAME | Name of the Google Pub/Sub subscription. Required for Google Pub/Sub. | |

**For MQTT:**

| Config | Description | Default Value |
|---|---|---|
| MQTT_PORT | Port of the MQTT broker | 1883 |
| MQTT_MESSAGE_ORDER | Enable guaranteed message order | false |
| MQTT_PROTOCOL | Communication protocol. Supported values: tcp, ssl. | tcp |
| MQTT_HOST | Hostname of the MQTT broker | localhost |
| MQTT_USER | Username for the MQTT broker | |
| MQTT_PASSWORD | Password for the MQTT broker | |
| MQTT_CLIENT_ID_SUFFIX | Suffix appended to the client ID | |

### Mongo Configs

| Config | Description | |
|---|---|---|
| MONGO_URI | URI for connecting to the MongoDB server. Required for MongoDB. | |
| MONGO_DATABASE | Name of the MongoDB database to use. Required for MongoDB. | |

### Redis Configs

| Config | Description | |
|---|---|---|
| REDIS_HOST | Hostname of the Redis server. | |
| REDIS_PORT | Port of the Redis server. | |

### SQL Configs

| Config | Description | Supported Values |
|---|---|---|
| DB_DIALECT | Database dialect. Required for SQL databases. | mysql, postgres |
| DB_HOST | Hostname of the database server. | |
| DB_PORT | Port of the database server. | 3306 |
| DB_USER | Username for the database. | |
| DB_PASSWORD | Password for the database. | |
| DB_NAME | Name of the database to use. | |

## HTTP Configs

| Config | Description | |
|---|---|---|
| REQUEST_TIMEOUT | Set the request timeouts (in seconds) for HTTP server. |  5 |

