# GoFr Configuration Options

This document lists all the configuration options supported by the Gofr framework. The configurations are grouped by category for better organization.

## App Configs

{% table %}

- Name: APP_NAME
- Description: Name of the application
- Default Value: gofr-app

---

- Name: APP_ENV
- Description: Name of the environment file to use (e.g., stage.env, prod.env, or local.env).

---

- Name: APP_VERSION
- Description: Application version
- Default Value: dev

---

- Name: LOG_LEVEL
- Description: Level of verbosity for application logs. Supported values are **DEBUG, INFO, NOTICE, WARN, ERROR, FATAL**
- Default Value: INFO

---

- Name: REMOTE_LOG_URL
- Description: URL to remotely change the log level

---

- Name: REMOTE_LOG_FETCH_INTERVAL
- Description: Time interval (in seconds) to check for remote log level updates
- Default Value: 15

---

- Name: METRICS_PORT
- Description: Port on which the application exposes metrics
- Default Value: 2121

---

- Name: HTTP_PORT
- Description: Port on which the HTTP server listens
- Default Value: 8000

---

- Name: GRPC_PORT
- Description: Port on which the gRPC server listens
- Default Value: 9000

---

- Name: TRACE_EXPORTER
- Description: Tracing exporter to use. Supported values: gofr, zipkin, jaeger.
- Default Value: gofr

---

- Name: TRACER_HOST
- Description: Hostname of the tracing collector. Required if TRACE_EXPORTER is set to zipkin or jaeger.

---

- Name: TRACER_PORT
- Description: Port of the tracing collector. Required if TRACE_EXPORTER is set to zipkin or jaeger.
- Default Value: 9411

---

- Name: CMD_LOGS_FILE
- Description: File to save the logs in case of a CMD application

{% endtable %}

## Datasource Configs

{% table %}

- Name: PUBSUB_BACKEND
- Description: Pub/Sub message broker backend
- Supported Values: kafka, google, mqtt

{% endtable %}

**For Kafka:**

{% table %}

- Name: PUBSUB_BROKER
- Description: Comma-separated list of broker addresses
- Default Value: localhost:9092

---

- Name: PARTITION_SIZE
- Description: Size of each message partition (in bytes)
- Default Value: 0

---

- Name: PUBSUB_OFFSET
- Description: Offset to start consuming messages from. -1 for earliest, 0 for latest.
- Default Value: -1

---

- Name: CONSUMER_ID
- Description: Unique identifier for this consumer
- Default Value: gofr-consumer

{% endtable %}

**For Google:**

{% table %}

- Name: GOOGLE_PROJECT_ID
- Description: ID of the Google Cloud project. Required for Google Pub/Sub.

---

- Name: GOOGLE_SUBSCRIPTION_NAME
- Description: Name of the Google Pub/Sub subscription. Required for Google Pub/Sub.

{% endtable %}

**For MQTT:**

{% table %}

- Name: MQTT_PORT
- Description: Port of the MQTT broker
- Default Value: 1883

---

- Name: MQTT_MESSAGE_ORDER
- Description: Enable guaranteed message order
- Default Value: false

---

- Name: MQTT_PROTOCOL
- Description: Communication protocol. Supported values: tcp, ssl.
- Default Value: tcp

---

- Name: MQTT_HOST
- Description: Hostname of the MQTT broker
- Default Value: localhost

---

- Name: MQTT_USER
- Description: Username for the MQTT broker

---

- Name: MQTT_PASSWORD
- Description: Password for the MQTT broker

---

- Name: MQTT_CLIENT_ID_SUFFIX
- Description: Suffix appended to the client ID

---

- Name: MQTT_QOS
- Description: Quality of Service Level

{% endtable %}

### Mongo Configs

{% table %}

- Name: MONGO_URI
- Description: URI for connecting to the MongoDB server.

---

- Name: MONGO_DATABASE
- Description: Name of the MongoDB database to use.

{% endtable %}

### Redis Configs

{% table %}

- Name: REDIS_HOST
- Description: Hostname of the Redis server.

---

- Name: REDIS_PORT
- Description: Port of the Redis server.

{% endtable %}

### SQL Configs

{% table %}

- Name: DB_DIALECT
- Description: Database dialect. Supported values: mysql, postgres

---

- Name: DB_HOST
- Description: Hostname of the database server.

---

- Name: DB_PORT
- Description: Port of the database server.
- Default Value: 3306

---

- Name: DB_USER
- Description: Username for the database.

---

- Name: DB_PASSWORD
- Description: Password for the database.

---

- Name: DB_NAME
- Description: Name of the database to use.

{% endtable %}

## HTTP Configs

{% table %}

- Name: REQUEST_TIMEOUT
- Description: Set the request timeouts (in seconds) for HTTP server.

{% endtable %}
