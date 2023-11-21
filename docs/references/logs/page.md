# Logs

GoFr supports level-based logging, which ensures that logs are generated only for events equal to or above the specified log level. The available log levels, in order of severity, are:

```bash
FATAL
ERROR
WARN
INFO (default)
DEBUG
```

The default logger is initialised when the gofr object is created.

## Changing Log Level

To change log level set LOG_LEVEL config in the .env file.
It will show logs of the specified level and higher are displayed in the output

```bash
LOG_LEVEL=WARN
```

## Using the Default Logger

```go
package main

import (
    "gofr.dev/pkg/gofr"
)

func main() {
    // Create the gofr object
    k := gofr.New()

    // Add a handler
    k.GET("/hello", func(c *gofr.Context) (interface{}, error) {
        c.Logger.Info("Testing Info logger" // Using the logger.
        return "Hello World!", nil
    })

    // Start the server
    k.Start()
}
```
