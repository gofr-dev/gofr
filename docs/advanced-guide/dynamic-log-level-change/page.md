# Dynamic Log Level Change

Gofr provides support for dynamic remote log level updates, allowing users to adjust the log level of their application on-the-go. 
This feature is facilitated through simple configuration settings.

## Configuration
To enable remote log level updating, users need to specify the following configuration parameter:

```dotenv
REMOTE_LOG_URL=<URL to remote log level endpoint>
```

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


GoFr will automatically fetch the response from this URL and then update the log level dynamically.

By default the time-interval between the request to fetch log level is `15 Seconds`. 

