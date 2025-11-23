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

### DEBUG
This is the lowest priority level (Integer value: 1). It represents the most detailed/granual information.

**Color -** Grey


#### **Usage Examples:**

***1. Variable States and Intermediate Values -***
It allows developers to verify that calculations, data transformations, and state changes are occurring exactly as intended before the final result is produced.

**Code Example**
	
```Go
// Context: Calculating a discount inside a shopping cart function
originalPrice := 150.00
discountRate := 0.20 // 20%
tax := 1.05          // 5% tax

logger.Debug("Calc trace - Price:", originalPrice, "Discount:", discountRate, "Tax Multiplier:", tax)

```

**Output**
```Console

DEBU [10:15:01] Calc trace - Price: 150 Discount: 0.2 Tax Multiplier: 1.05
```

***2. Control Flow and Loop Iterations -***
 Conditional logic (if/else statements), and the step-by-step progress of iterative loops.

**Code Example**

```Go
// Context: Processing a batch of user IDs
userIds := []int{101, 102, 103}

logger.Debug("Starting batch processing for", len(userIds), "users")

for i, id := range userIds {
	logger.Debug("Loop step", i, "- Processing User ID:", id)
}
```
**Output**
```Console
DEBU [10:15:02] Starting batch processing for 3 users
DEBU [10:15:02] Loop step 0 - Processing User ID: 101
DEBU [10:15:02] Loop step 1 - Processing User ID: 102
DEBU [10:15:02] Loop step 2 - Processing User ID: 103
```

***3. Raw Data Payloads,Initialization and Database Internals -*** Used to debug API integrations, Initialization, Processing.

**Code Example**
```Go
// 1. Resource Initialization (Startup)
logger.Debug("[Init] Loading config from ./config.yaml")

// 2. Raw Data Payload (Input)
data := `{"id": 42, "role": "admin"}`
logger.Debug("[Payload] Received raw body:", data)

// 3. Database Internals (Processing)
query := fmt.Sprintf("SELECT * FROM users WHERE id=%d", 42)
logger.Debug("[SQL] Generated Query:", query)
```
**Output**
```Console
DEBU [10:15:05] [Init] Loading config from ./config.yaml
DEBU [10:15:05] [Payload] Received raw body: {"id": 42, "role": "admin"}
DEBU [10:15:05] [SQL] Generated Query: SELECT * FROM users WHERE id=42
```

#### **Examples of when Not to Use:**


***1. Avoid Logging Critical Business Data:*** Do not utilize `DEBUG` for audit trails or essential transaction records. In production environments, this level is typically suppressed to optimize performance, resulting in the loss of critical business insights.

***2. Prohibit PII Logging:*** Strictly avoid logging Personally Identifiable Information (PII) or credentials (e.g., passwords, tokens)As `DEBUG` is frequently used for raw variable dumps, there is a high risk of exposing sensitive data in plain text

---
### INFO
Represents standard operational events (Integer value: 2). It is the default fallback level if an unknown level string is provided.

**Color -** Cyan

#### **Usage Examples:**

***1. Server Starting -*** Initiating the application boot sequence and binding to the configured network ports.


**Code Example**
```Go
// Example: Server startup confirmation
logger.Info("Server started successfully on port 8000")
```
**Output-**
```Console
INFO [14:05:02] Server started successfully on port 8000
```
***2. Job Completed -*** The scheduled background task has successfully finished execution and released all locked resources.


**Code Example**
```Go
// Context: A background data export job has finished successfully
jobID := "EXP-2024-88"
recordsProcessed := 5000
duration := "1.2s"

logger.Info("Data export job completed successfully",
	"JobID", jobID, 
    "Records", recordsProcessed, 
    "Duration", duration)
```
**Output**
```Console
INFO [14:20:05] Data export job completed successfully JobID: EXP-2024-88 Records: 5000 Duration: 1.2s
```

***3. Health Check Passed -*** Routine diagnostic monitoring confirms that all system services are active and responding within normal parameters.


**Code Example**
```Go
// Log an INFO level message with a key-value pair
logger.Info("Health Check Passed")
```
**Output**
```Console
INFO [14:05:02] Health Check Passed
```

#### **Examples of when not to use**

***1. Do Not Use for Exceptions:*** Refrain from using this level for error conditions.`INFO` logs are routed to standard output (`stdout`), causing them to be potentially overlooked by monitoring tools specifically configured to capture standard error (`stderr`) streams.

***2. Avoid High-Frequency Saturation:*** 
Do not emit `INFO` logs within tight loops or data-intensive processing blocks. Excessive logging at this level can rapidly saturate storage and obscure significant operational events with noise.
---

### NOTICE
A level higher than `INFO` but lower than `WARN` (Integer value: 3). It shares the same visual prominence as a Warning but implies a "normal" condition rather than a problem. in simple words its used for events that are normal but rare and significant.

**Color -** Yellow

#### **Usage Examples:**

***1. Configuration Reloaded -*** System settings have been hot-swapped dynamically without requiring a full application restart.

**Code Example**
```Go
// Example: Configuration update
logger.Notice("Configuration hot-reload triggered by system admin")
```
**Output-**
```Console
NOTI [14:05:03] Configuration hot-reload triggered by system admin
```
***2. Switching to Secondary Database -*** Primary node connectivity was lost, so traffic is being automatically rerouted to the replica instance to maintain uptime.


**Code Example**
```Go
// Using the classic logger to manually tag the level
logger.Notice("Switching to Secondary Database")
```
**Output**
```Console
NOTI [14:52:00] Switching to Secondary Database
```

***3. Cache Cleared -*** The in-memory data store has been purged to ensure subsequent requests fetch fresh data directly from the source.


**Code Example**
```Go
logger.Notice("Cache Cleared")
```
**Output**
```Console
NOTI [14:52:00] Cache Cleared
```

#### **Examples of when not to use**

***1. Misclassification of Failures:*** Do not utilize this level for error scenarios. `NOTICE` semantically implies a healthy system state; using it for failures creates ambiguity regarding system health.

***2. Avoid Routine Operations:*** 
Do not apply this level to standard, high-volume request logs. `NOTICE `should be reserved for distinct, infrequent state changes rather than repetitive per-request activities.

---

### WARN
it's Used when an anomaly had occurred, but the application recovered or continued execution.

**Color -** Yellow

#### **Usage Examples:**

***1. Retrying database connection -*** Temporary connectivity loss detected. initiating an exponential backoff strategy to re-establish the link.

**Code Example**
```Go
// Example: Retrying a connection
logger.Warn("Database connection timeout. Retrying in 2 seconds... (Attempt 1/3)")
```
**Output**
```Console
WARN [14:05:04] Database connection timeout. Retrying in 2 seconds... (Attempt 1/3)
```
***2. Using fallback values -*** The external configuration service is unreachable, so the system is defaulting to hardcoded safe parameters to continue operation.


**Code Example**
```Go
logger.Warn("Timeout config not found. Using fallback: 30s")
```
**Output**
```Console
WARN [14:55:00] Timeout config not found. Using fallback: 30s
```

***3. Deprecated API usage -*** The application is calling an obsolete function or endpoint that will be removed in future versions; code migration is required.


**Code Example**
```Go
logger.Warn("Deprecated API usage detected: /v1/login")
```
**Output**
```Console
WARN [14:56:00] Deprecated API usage detected: /v1/login
```

#### **Examples of when not to use**

***1. Do Not Use for Definitive Failures:*** If a specific request or operation fails completely, do not categorize it as a `WARN`. This level implies the system "survived" or handled the issue gracefully; unrecoverable failures should be logged as `ERROR`.

***2. Avoid Precautionary Logging:*** 
Do not log warnings for standard behaviors or expected redundancies (false positives). Overuse desensitizes operators to genuine alerts.


---

### ERROR
Indicates a failure event (Integer value: 5).This level routes logs to `stderr` (Standard Error), ensuring visibility to error tracking tools.

**Color -** Red

#### **Usage Examples:**

***1. database timeouts -*** The database query exceeded the maximum execution time limit and was forcibly cancelled to prevent resource exhaustion.

**Code Example**
```Go
// Context: A complex query exceeds the defined execution time limit
// Simulating a context deadline exceeded error
err := context.DeadlineExceeded

if err != nil {
    logger.Error("DB Query Timeout: Analytics fetch took > 3000ms. Canceling operation.")
}
```
**Output**
```Console
ERRO [10:20:01] DB Query Timeout: Analytics fetch took > 3000ms. Canceling operation.
```
***2. 500 Internal Server Errors -*** An unexpected condition was encountered on the server side that prevented it from fulfilling the incoming request.


**Code Example**
```Go
// Context: An API endpoint fails to process a request due to a downstream failure
err := processPayment() // returns error: "gateway unreachable"

if err != nil {
    // We send a generic 500 to the user, but log the specific error internally
    logger.Error("HTTP 500 Response: Payment gateway unreachable. Request ID: req_99")
}
```
**Output**
```Console
ERRO [10:20:02] HTTP 500 Response: Payment gateway unreachable. Request ID: req_99
```

***3. null pointer exceptions -*** The code attempted to dereference a memory address that does not point to a valid object, causing a runtime panic.


**Code Example**
```Go
// Context: Preventing a panic by checking if a struct is nil before accessing it
var userProfile *User // This is currently nil

if userProfile == nil {
    logger.Error("Runtime Safety: Attempted to access methods on a nil 'User' object. Skipping.")
}
```
**Output**
```Console
ERRO [10:20:03] Runtime Safety: Attempted to access methods on a nil 'User' object. Skipping.
```

#### **Examples of when not to use**

***1. Inappropriate for System-Wide Crashes:*** Do not use `ERROR` for unrecoverable startup failures that render the application non-functional (e.g., missing critical configuration). Such dependencies must be handled via `FATAL` to ensure immediate process termination

***2. Avoid Logging Client-Side Validation as System Errors*** 
Exercise caution when logging user input errors (e.g., `400 Bad Request`). Classifying routine client-side validation failures as system ERRORs creates noise in alerting systems; `INFO` or `WARN` is often more

---

### FATAL
The highest priority level (Integer value: 6). It represents a critical system failures where the application cannot function.

**Color -** Red

#### **Usage Examples:**

***1. Port already in use -*** The application failed to bind to the network socket because another process is currently listening on the specified port.

**Code Example**
```Go
// Context: The web server attempts to start, but the port is occupied
port := ":8080"
err := http.ListenAndServe(port, nil)

if err != nil {
    // The application cannot run without a network listener, so we crash
    logger.Fatal("Network Bind Failure: Port 8080 is already in use by another process.")
}
```
**Output**
```Console
FATA [10:30:01] Network Bind Failure: Port 8080 is already in use by another process.
```
***2. Missing encryption keys -*** Essential security credentials required for signing tokens or encrypting data are absent from the environment variables.


**Code Example**
```Go
// Context: Checking environment variables for security keys before starting
jwtKey := os.Getenv("JWT_PRIVATE_KEY")

if jwtKey == "" {
    // We cannot start the app insecurely, so we force a shutdown
    logger.Fatal("SECURITY CRITICAL: Missing encryption keys. JWT_PRIVATE_KEY is empty.")
}
```
**Output**
```Console
FATA [10:30:02] SECURITY CRITICAL: Missing encryption keys. JWT_PRIVATE_KEY is empty.
```

***3. Cannot connect to primary database on startup -*** The application is aborting the boot sequence because it cannot establish an initial connection to the required data store.


**Code Example**
```Go
// Context: Initial "Ping" to the database during the boot sequence
err := db.Ping()

if err != nil {
    // Unlike a runtime error, if the DB is gone at startup, the app is useless
    logger.Fatal("Boot Failure: Cannot connect to primary database. Connection refused.")
}
```
**Output**
```Console
FATA [10:30:03] Boot Failure: Cannot connect to primary database. Connection refused.
```

#### **Examples when not to use**

***1. Strictly Prohibited During Request Handling:*** Never invoke `FATAL` during runtime request processing. This method executes `os.Exit(1)`, causing the entire server instance to terminate immediately. Using this for a runtime error (like a failed SQL query) causes a complete service outage rather than a single request failure.


---
> **Note:** Performance & Log Volume.
>1. Early Exit Optimization: The logger implements an "Early Exit" strategy. If the incoming log level is lower than the configured `LOG_LEVEL`, the function returns immediately before performing any formatting or allocation.
>2. Locking Overhead: The terminal output utilizes a mutex lock to ensure thread safety.

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
