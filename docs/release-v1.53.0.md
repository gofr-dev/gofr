# **Release v1.53.0**

## 🚀 Features

🔹 **Amazon SQS PubSub Support**

GoFr now supports **Amazon SQS** as a messaging backend, enabling seamless integration with AWS Simple Queue Service for building resilient, distributed microservices.

- Supports both **Publishing** and **Subscribing** to SQS queues
- Integrated **Health Checks** to monitor SQS connection status
- **Full Observability**:
  - Automatic logging of message publishing and processing
  - Distributed tracing for end-to-end message tracking
  - Metrics for message throughput and error rates
- **Configurable Behavior**:
  - Support for custom message attributes
  - Configurable visibility timeouts and wait times
  - Support for both standard and FIFO queues
- Seamless integration with GoFr's `PubSub` interface for easy backend switching

#### **Usage Example**

To use Amazon SQS, import the driver and add it to your application:

```go
import (
    "gofr.dev/pkg/gofr"
    "gofr.dev/pkg/gofr/datasource/pubsub/sqs"
)

func main() {
    app := gofr.New()

    app.AddPubSub(sqs.New(&sqs.Config{
        Region: "us-east-1",
    }))

    app.Subscribe("my-queue", func(ctx *gofr.Context) error {
        // Process message
        return nil
    })

    app.Run()
}
```

---

## 🚀 Enhancements

🔹 **Metrics for HTTP Service Resilience**

Enhanced observability for inter-service communication by adding dedicated metrics for resilience patterns.

- New metric `app_http_retry_count`: Tracks the total number of retries performed for each downstream service
- New metric `app_http_circuit_breaker_state`: A gauge tracking the current state (0 for Closed, 1 for Open) of the circuit breaker
- **Updated Grafana Dashboards**: Included new panels to visualize retry events and circuit breaker transitions, providing immediate insights into service stability and failure patterns.

🔹 **Configurable Metrics Server**

Added the ability to **disable the internal metrics server** by setting the `METRICS_PORT` environment variable to `0`. This provides greater flexibility for users who prefer to handle metrics collection through external agents or in environments where a separate metrics port is not required.

🔹 **Zip Slip Vulnerability Protection**

Improved the security of the `file/zip` package by implementing protection against **Zip Slip** (path traversal) attacks.

- Automatically validates file paths during extraction to ensure they remain within the target directory
- Rejects zip entries containing absolute paths or parent directory traversal sequences (`..`)
- Ensures safe handling of compressed files from untrusted sources

🔹 **Dependency Updates**

Updated several core dependencies to their latest versions to leverage performance improvements and security patches:
- AWS SDK (v2)
- NATS.go
- Google API Clients
- BadgerDB (v4)

---

## 🛠️ Fixes

🔹 **Documentation Formatting**

Fixed formatting and layout issues in the **HTTP Communication** documentation page. These improvements ensure that code examples and configuration guides are rendered correctly, enhancing the overall developer experience.

🔹 **Test Reliability Improvements**

Refactored internal test helpers to use `t.Cleanup` instead of `defer`. This ensures more reliable resource cleanup during tests and prevents potential leaks or interference between test cases, leading to a more stable CI/CD pipeline.
