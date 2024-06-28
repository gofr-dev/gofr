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

> NOTE: If not provided the default interval between the request to fetch log level is **15 seconds**.

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
