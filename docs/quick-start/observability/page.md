# Observability

GoFr, by default, manages observability in different ways once the server starts:

## Logs
Logs offer real-time information, providing valuable insights and immediate visibility into the ongoing state and activities of the system.
It helps in identifying errors, debugging and troubleshooting, monitor performance, analyzing application usage, communications etc.

GoFr logger allows customizing the log level, which provides flexibility to adjust logs based on specific needs.

Logs are generated only for events equal to or above the specified log level; by default, GoFr logs at _INFO_ level.
Log Level can be changed by setting the environment variable `LOG_LEVEL` value to _DEBUG, INFO, NOTICE, WARN, ERROR or FATAL_.

When the GoFr server runs, it prints a log for reading configs, database connection, requests, database queries, missing configs, etc.
They contain information such as request's correlation ID, status codes, request time, etc.

### DEBUG
This is the lowest priority level. It represents the most detailed/granular information.

**Note:** `DEBUG` logs should be enabled only in development or controlled troubleshooting scenarios.They are typically disabled in production environments due to performance overhead and security risks.


#### **Usage Examples:**

***1. Variable States and Intermediate Values -***
It allows developers to verify that calculations, data transformations, and state changes are occurring exactly as intended before the final result is produced.

**Code Example**
	
```Go
// CalculateDiscount is a handler that calculates the final price
func CalculateDiscount(ctx *gofr.Context) (interface{}, error) {
    originalPrice := 150.00
    discountRate := 0.20 // 20%
    tax := 1.05          // 5% tax

    ctx.Debug("Calc trace - Price:", originalPrice, "Discount:", discountRate, "Tax Multiplier:", tax)
    
    return nil, nil
}
```

**Output**
```Console

DEBU [10:15:01] Calc trace - Price: 150 Discount: 0.2 Tax Multiplier: 1.05
```

***2. Control Flow and Loop Iterations -***
 Conditional logic (if/else statements), and the step-by-step progress of iterative loops.

**Code Example**

```Go
// ProcessBatch simulates processing a list of users
func ProcessBatch(ctx *gofr.Context) (interface{}, error) {
    userIds := []int{101, 102, 103}

    ctx.Debug("Starting batch processing for", len(userIds), "users")

    for i, id := range userIds {
        ctx.Debug("Loop step", i, "- Processing User ID:", id)
    }
    return nil, nil
}
```
**Output**
```Console
DEBU [10:15:02] Starting batch processing for 3 users
DEBU [10:15:02] Loop step 0 - Processing User ID: 101
DEBU [10:15:02] Loop step 1 - Processing User ID: 102
DEBU [10:15:02] Loop step 2 - Processing User ID: 103
```

***3. Raw Data Payloads, Initialization and Database Internals -*** Used to debug API integrations, Initialization, Processing.

> **Security Warning:** Logging raw request payloads is intended only for development or controlled debugging. Never enable such logs in production environments if payloads may contain Personally Identifiable Information (PII), credentials, authentication tokens, or other sensitive data.


**Code Example**
```Go
// InspectPayload simulates debugging an incoming request payload
func InspectPayload(ctx *gofr.Context) (interface{}, error) {
    // 1. Raw Data Payload (Input)
    data := `{"id": 42, "role": "admin"}`
    ctx.Debug("[Payload] Received raw body:", data)

    // 2. Database Internals (Processing)
    query := fmt.Sprintf("SELECT * FROM users WHERE id=%d", 42)
    ctx.Debug("[SQL] Generated Query:", query)
    
    return nil, nil
}
```
**Output**
```Console
DEBU [10:15:05] [Payload] Received raw body: {"id": 42, "role": "admin"}
DEBU [10:15:05] [SQL] Generated Query: SELECT * FROM users WHERE id=42
```

#### **Examples of when Not to Use:**

***1. Redundant Framework Logging:*** Avoid manually logging information that GoFr already captures at the `DEBUG` level, such as raw SQL queries or basic request/response details, to prevent log duplication and unnecessary verbosity.

***2. Prohibit PII Logging:*** Strictly avoid logging Personally Identifiable Information (PII) or credentials (e.g., passwords, tokens). As `DEBUG` is frequently used for raw variable dumps, there is a high risk of exposing sensitive data in plain text.

---
### INFO
`INFO` Represents normal operational events during application execution and acts as the default logging level, ensuring baseline observability without excessive verbosity.



#### **Usage Examples:**

***1. Configuration Loading -*** Confirms successful loading and validation of application configuration.


**Code Example**
```Go
// Log after application configuration is read and validated
func LoadConfig(ctx *gofr.Context) {
    configSource := "env"

    ctx.Info(
        "Application configuration loaded and validated",
        "Source", configSource,
    )
}

```
**Output**
```Console
INFO [10:02:15] Application configuration loaded and validated Source: env

```
***2. Database Ready -*** Indicates that the database connection has been successfully established.


**Code Example**
```Go
// Log after a successful database connection setup
func InitDatabase(ctx *gofr.Context) {
    dbHost := "localhost"

    ctx.Info(
        "Database connection is ready",
        "Host", dbHost,
    )
}


```
**Output**
```Console
INFO [10:02:18] Database connection is ready Host: localhost
```

***3. Cache Initialized -*** Indicates that the cache client is ready and available.

**Code Example**
```Go
// Log after cache client Initialization
func InitCache(ctx *gofr.Context) {
    cacheType := "redis"

    ctx.Info(
        "Cache client initialized successfully",
        "Type", cacheType,
    )
}
```
**Output**
```Console
INFO [10:02:20] Cache client initialized successfully Type: redis

```

#### **Examples of when Not to Use:**

***1. Do Not Use for Exceptions:*** Refrain from using this level for error conditions.`INFO` logs are routed to standard output (`stdout`), causing them to be potentially overlooked by monitoring tools specifically configured to capture standard error (`stderr`) streams.

***2. Avoid High-Frequency Saturation:*** 
Do not emit `INFO` logs within tight loops or data-intensive processing blocks. Excessive logging at this level can rapidly saturate storage and obscure significant operational events with noise.

---

### NOTICE
A level higher than `INFO` but lower than `WARN`. It shares the same visual prominence as a Warning but implies a "normal" condition rather than a problem. In simple words, it's used for events that are normal but rare and significant.



#### **Usage Examples:**

***1. Configuration Reloaded -*** System settings have been hot-swapped dynamically without requiring a full application restart.

**Code Example**
```Go
// TriggerReload is an admin handler to refresh configs
func TriggerReload(ctx *gofr.Context) (interface{}, error) {
    ctx.Notice("Configuration hot-reload triggered by system admin")
    return "Config Reloaded", nil
}
```
**Output**
```Console
NOTI [14:05:03] Configuration hot-reload triggered by system admin
```
***2. Switching to Secondary Database -*** Primary node connectivity was lost, so traffic is being automatically rerouted to the replica instance to maintain uptime.


**Code Example**
```Go
// CheckDBConnection monitors database status
func CheckDBConnection(ctx *gofr.Context) (interface{}, error) {
    // Logic to detect primary DB failure...
    ctx.Notice("Switching to Secondary Database")
    return nil, nil
}
```
**Output**
```Console
NOTI [14:52:00] Switching to Secondary Database
```

***3. Cache Cleared -*** The in-memory data store has been purged to ensure subsequent requests fetch fresh data directly from the source.


**Code Example**
```Go
// InvalidateCache clears the application cache
func InvalidateCache(ctx *gofr.Context) (interface{}, error) {
    ctx.Notice("Cache Cleared")
    return nil, nil
}
```
**Output**
```Console
NOTI [14:52:00] Cache Cleared
```

#### **Examples of when Not to Use:**

***1. Misclassification of Failures:*** Do not utilize this level for error scenarios. `NOTICE` semantically implies a healthy system state; using it for failures creates ambiguity regarding system health.

***2. Avoid Routine Operations:*** 
Do not apply this level to standard, high-volume request logs. `NOTICE `should be reserved for distinct, infrequent state changes rather than repetitive per-request activities.

---
### WARN
`WARN` should represent abnormal runtime conditions that indicate instability or degraded operation (retries, fallbacks, transient failures), not long-term code hygiene issues like deprecated API usage. If something would show up repeatedly in a healthy system, it shouldn’t be a `WARN`, otherwise the signal gets diluted and operators start ignoring it.



#### **Usage Examples:**

***1. Database Connection Retry -*** Temporary connectivity loss detected. Initiating an exponential backoff strategy to re-establish the link.

**Code Example**
```Go
// ConnectWithRetry simulates a resilient database connection
func ConnectWithRetry(ctx *gofr.Context) (interface{}, error) {
    // Simulating a failed attempt
    ctx.Warn("Database connection timeout. Retrying...", "attempt", 1, "retry_after", "2s")
    return nil, nil
}
```
**Output**
```Console
WARN [14:05:04] Database connection timeout. Retrying... attempt: 1 retry_after: 2s
```
***2. Fallback Configuration Used -*** The external configuration service is unreachable, so the system is defaulting to hardcoded safe parameters to continue operation.


**Code Example**
```Go
// GetTimeoutConfig retrieves config with a safe fallback
func GetTimeoutConfig(ctx *gofr.Context) (interface{}, error) {
    ctx.Warn("Timeout config not found. Using fallback.", "fallback_value", "30s")
    return 30, nil
}
```
**Output**
```Console
WARN [14:55:00] Timeout config not found. Using fallback. fallback_value: 30s
```

#### **Examples of when Not to Use:**

***1. Do Not Use for Definitive Failures:*** If a specific request or operation fails completely, do not categorize it as a `WARN`. This level implies the system "survived" or handled the issue gracefully; unrecoverable failures should be logged as `ERROR`.

***2. Avoid Precautionary Logging:*** 
Do not log warnings for standard behaviors or expected redundancies (false positives). Overuse desensitizes operators to genuine alerts.


---

### ERROR
Indicates a failure event. This level routes logs to `stderr` (Standard Error), ensuring visibility to error tracking tools.



#### **Usage Examples:**

***1. Database Timeouts -*** The database query exceeded the maximum execution time limit and was forcibly cancelled to prevent resource exhaustion.

**Code Example**
```Go
// FetchAnalytics simulates a long-running query that times out
func FetchAnalytics(ctx *gofr.Context) (interface{}, error) {
    // Logic to fetch analytics...
    err := errors.New("query execution exceeded 3000ms")
    
    ctx.Error("DB Query Timeout: Analytics fetch failed.", "error", err)
    
    return nil, err
}
```
**Output**
```Console
ERROR [10:20:01] DB Query Timeout: Analytics fetch failed. error: query execution exceeded 3000ms
```
***2. External Service Failure -*** An unexpected condition was encountered when calling a downstream service that prevented it from fulfilling the request.


**Code Example**
```Go
// ProcessPayment simulates a downstream service failure
func ProcessPayment(ctx *gofr.Context) (interface{}, error) {
    // Simulating a gateway failure
    err := errors.New("payment gateway unreachable")
    
    ctx.Error("Payment processing failed.", "error", err, "request_id", "req_99")
    
    return nil, err
}
```
**Output**
```Console
ERROR [10:20:02] Payment processing failed. error: payment gateway unreachable request_id: req_99
```

***3. Null Pointer Exceptions -*** The code attempted to dereference a memory address that does not point to a valid object, causing a runtime panic.


**Code Example**
```Go
// GetUserProfile retrieves user data safely
func GetUserProfile(ctx *gofr.Context) (interface{}, error) {
    var userProfile *User // Currently nil
    
    if userProfile == nil {
        ctx.Error("Runtime Safety: Attempted to access methods on a nil 'User' object. Skipping.")
        return nil, fmt.Errorf("user not found")
    }
    return userProfile, nil
}
```
**Output**
```Console
ERROR [10:20:03] Runtime Safety: Attempted to access methods on a nil 'User' object. Skipping.
```

#### **Examples of when Not to Use:**

***1. Inappropriate for System-Wide Crashes:*** Do not use `ERROR` for unrecoverable startup failures that render the application non-functional (e.g., missing critical configuration). Such dependencies must be handled via `FATAL` to ensure immediate process termination

***2. Avoid Logging Client-Side Validation as System ERROR*** 
Exercise caution when logging user input error (e.g., `400 Bad Request`). Classifying routine client-side validation failures as system ERROR creates noise in alerting systems; `INFO` or `WARN` is often more

---

### FATAL
The highest priority level. `FATAL` represents a critical system failures where the application cannot function. 

**Note:** `FATAL` terminates the process immediately and is intended only for startup-time failures, not runtime request handling.


#### **Usage Examples:**

***1. Missing Critical Resource -*** The application cannot start because a mandatory resource, such as a cryptographic certificate or a required local file, is missing.

**Code Example**
```Go
// Context: Checking for a mandatory certificate before starting
if _, err := os.Stat("/etc/certs/server.crt"); os.IsNotExist(err) {
    app.Logger().Fatal("Startup Failure: Mandatory SSL certificate missing.", "path", "/etc/certs/server.crt")
}
```
**Output**
```Console
FATA [10:30:01] Startup Failure: Mandatory SSL certificate missing. path: /etc/certs/server.crt
```
***2. Incompatible Environment -*** The application requires a specific environment or dependency version to function correctly and must shut down if it's not met.


**Code Example**
```Go
// Context: Verifying a required system dependency
currentVersion := os.Getenv("DEP_VERSION")
if !isSupportedVersion(currentVersion) {
    app.Logger().Fatal("Incompatible Environment.", "required_version", "2.0", "current_version", currentVersion)
}
```
**Output**
```Console
FATA [10:30:02] Incompatible Environment. required_version: 2.0 current_version: 1.5
```

#### **Examples of when Not to Use:**

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

### Default Metrics

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

---

- app_http_retry_count
- counter
- Total number of retry events

---

- app_http_circuit_breaker_state
- gauge
- Current state of the circuit breaker (0 for Closed, 1 for Open). Used for historical timeline visualization.

{% /table %}

For example: When running the application locally, we can access the /metrics endpoint on port 2121 from: {% new-tab-link title="http://localhost:2121/metrics" href="http://localhost:2121/metrics" /%}

GoFr also supports creating {% new-tab-link newtab=false title="custom metrics" href="/docs/advanced-guide/publishing-custom-metrics" /%}.

### Disabling the Metrics Server

To disable the metrics server entirely, set the `METRICS_PORT` environment variable to `0`:

```dotenv
METRICS_PORT=0
```

### Example Dashboard

These metrics can be easily consumed by monitoring systems like {% new-tab-link title="Prometheus" href="https://prometheus.io/" /%}
and visualized in dashboards using tools like {% new-tab-link title="Grafana" href="https://grafana.com/" /%}.

You can find the dashboard source in the {% new-tab-link title="GoFr repository" href="https://github.com/gofr-dev/gofr/tree/main/examples/http-server/docker/provisioning/dashboards/gofr-dashboard" /%}.

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


### Custom Authentication Headers

Many observability platforms require custom headers for authentication. GoFr supports this through the `TRACER_HEADERS` configuration, which accepts comma-separated `key=value` pairs following the OpenTelemetry standard format.

#### Usage Examples

**Single Header:**
```dotenv
# Honeycomb
TRACER_HEADERS="X-Honeycomb-Team=your_api_key"
```

**Multiple Headers:**
```dotenv
# Grafana Cloud with multiple headers
TRACER_HEADERS="Authorization=Basic base64encodedcreds,X-Scope-OrgID=tenant-1"
```

```dotenv
# API key with special characters
TRACER_HEADERS="X-Api-Key=secret123,Authorization=Bearer token"
```

####  Configuration Example

Here's an example for sending traces to Grafana Cloud with authentication:

```dotenv
APP_NAME=my-service

# Grafana Cloud OTLP endpoint with authentication
TRACE_EXPORTER=otlp
TRACER_URL=otlp-gateway-prod-us-east-0.grafana.net:443
TRACER_HEADERS="Authorization=Basic dXNlcm5hbWU6cGFzc3dvcmQ=,X-Scope-OrgID=123456"
TRACER_RATIO=1.0
```
