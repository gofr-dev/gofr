# Remote Log Level Change
Gofr makes it easy to adjust the detail captured in your application's logs, even while it's running! This feature empowers 
users to effortlessly fine-tune logging levels without the need for redeployment, enhancing the monitoring and debugging experience.
This feature is facilitated through simple configuration settings.

## Why it is important?
- **Effortless Adjustments:** Modify the level of detail in your logs anytime without restarting your application. 
    This is especially helpful during troubleshooting or when log volume needs to be adjusted based on the situation.
- **Enhanced Visibility:** Easily switch to a more detailed log level (e.g., `DEBUG`) to gain deeper insights into specific issues, 
                       and then switch back to a less detailed level (e.g., `INFO`) for regular operation.

## Configuration
To enable remote log level updating, users need to specify the following configuration parameter:

```dotenv
REMOTE_LOG_URL=<URL to your remote log level endpoint> (e.g., https://your-service.com/log-levels)
REMOTE_LOG_FETCH_INTERVAL=<Interval in seconds> (default: 15)
```

- **REMOTE_LOG_URL:** Specifies the URL of the remote log level endpoint.
- **REMOTE_LOG_FETCH_INTERVAL:** Defines the time interval (in seconds) at which GoFr fetches log level configurations from the endpoint.

> NOTE: If not provided the default interval between the request to fetch log level is **15 seconds**.

## Remote Log Level Endpoint
The remote log level endpoint should return a JSON response in the following format:

```json
{
  "data": [
    {
      "serviceName": "sample-service",
      "logLevel": "DEBUG"
    }
  ]
}
```

- **serviceName:** Identifies the service for which log levels are configured.
- **logLevel:** The new log level you want to set for the specified service.


GoFr intelligently parses this response and dynamically adjusts log levels according to the provided configurations.

 

