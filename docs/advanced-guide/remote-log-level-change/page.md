# Remote Log Level Change

GoFr makes it easy to adjust the details captured in the application's logs, even while it's running!

This feature allows users to effortlessly fine-tune logging levels without the need for redeployment, enhancing the monitoring and debugging experience.
It is facilitated through simple configuration settings.

## How it helps?

- **Effortless Adjustments:** Modify the log level anytime without restarting the application. This is especially helpful during troubleshooting.
- **Enhanced Visibility:** Easily switch to a more detailed log level (e.g., `DEBUG`) to gain deeper insights into specific issues,
  and then switch back to a less detailed level (e.g., `INFO`) for regular operation.
- **Improved Performance:** Generating a large number of logs can overwhelm the logging system, leading to increased I/O operations and resource consumption,
  changing to Warn or Error Level reduces the number of logs, and enhancing performance.

## Configuration

To enable remote log level update, users need to specify the following configuration parameter:

```dotenv
REMOTE_LOG_URL=<URL to user's remote log level endpoint> (e.g., https://log-service.com/log-levels)
REMOTE_LOG_FETCH_INTERVAL=<Interval in seconds> (default: 15)
```

- **REMOTE_LOG_URL:** Specifies the URL of the remote log level endpoint.
- **REMOTE_LOG_FETCH_INTERVAL:** Defines the time interval (in seconds) at which GoFr fetches log level configurations from the endpoint.

> [!NOTE]
> If not provided the default interval between the request to fetch log level is **15 seconds**.

## Remote Log Level Endpoint

The remote log level endpoint should return a JSON response in the following format:

```json
{
  "data": {
    "serviceName": "test-service",
    "logLevel": "DEBUG"
  }
}
```

- **serviceName:** Identifies the service for which log levels are configured.
- **logLevel:** The new log level user want to set for the specified service.

GoFr parses this response and adjusts log levels based on the provided configurations.

## Advanced Usage: Implementing Custom Remote Configuration

GoFr provides a flexible way to handle remote configurations through two main interfaces: `Provider` and `Subscriber`. This allows you to create your own configuration providers or subscribe to configuration changes for custom logic.

### The `Provider` Interface

A `Provider` is responsible for fetching the configuration from a source and distributing it to its subscribers. It is defined as follows:

```go
// Provider represents a runtime config provider.
type Provider interface {
    Register(c Subscriber)
    Start()
}
```

- `Register(c Subscriber)`: Adds a new `Subscriber` to the provider's list.
- `Start()`: Starts the provider, which will begin fetching configurations and notifying subscribers.

GoFr includes an HTTP-based `Provider` called `httpRemoteConfig` which is created by `NewHTTPRemoteConfig`.

### The `Subscriber` Interface

A `Subscriber` is any component that can be updated at runtime. It is defined as follows:

```go
// Subscriber represents any component that can be updated at runtime.
type Subscriber interface {
    UpdateConfig(config map[string]any)
}
```

- `UpdateConfig(config map[string]any)`: This method is called by the `Provider` whenever there is a configuration update.

### Example: Creating a Custom Subscriber

You can create your own custom `Subscriber` to perform actions based on configuration changes. Here is an example of a `Subscriber` that toggles a feature flag.

```go
package main

import (
    "fmt"
    "log"
    "time"

    "gofr.dev/pkg/gofr/config"
    "gofr.dev/pkg/gofr/logging"
    "gofr.dev/pkg/gofr/logging/remotelogger"
)

// FeatureToggle is a custom subscriber that enables or disables a feature.
type FeatureToggle struct {
    enabled bool
}

// UpdateConfig is called when the configuration is updated.
func (ft *FeatureToggle) UpdateConfig(cfg map[string]any) {
    if enabled, ok := cfg["featureEnabled"].(bool); ok {
        ft.enabled = enabled
        log.Printf("Feature flag set to: %v", ft.enabled)
    }
}

func main() {
    // Create a new logger
    logger := logging.NewLogger(logging.INFO)

    // Create a new HTTP-based configuration provider.
    // This would typically be your own remote configuration endpoint.
    // For this example, we'll use a mock server.
    // In a real application, you would use your actual remote config URL.
    remoteConfigURL := "http://localhost:8080/config"
    provider := remotelogger.NewHTTPRemoteConfig(remoteConfigURL, 15*time.Second, logger)

    // Create and register our custom subscriber.
    featureToggler := &FeatureToggle{}
    provider.Register(featureToggler)

    // Start the provider.
    provider.Start()

    // Keep the application running.
    select {}
}
```

In this example:

1.  We define a `FeatureToggle` struct that implements the `Subscriber` interface.
2.  The `UpdateConfig` method checks for a `featureEnabled` key in the configuration and updates the `enabled` field.
3.  We create an instance of `httpRemoteConfig` which acts as our `Provider`.
4.  We register our `featureToggler` with the `provider`.
5.  We start the `provider`, which will then periodically fetch the configuration and update our `featureToggler`.