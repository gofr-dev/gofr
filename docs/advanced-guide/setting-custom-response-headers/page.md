# Custom Response Headers and Metadata in GoFr

GoFr simplifies the process of adding custom HTTP response headers and metadata to API responses using the `Response` struct. This feature allows you to include additional information such as custom headers or metadata to enhance client-server communication while keeping your data payload clean and structured.

## Features

1. **Custom Headers**: Add key-value pairs for headers, useful for:
    - Security policies
    - Debugging information
    - Versioning details

   **Type**: `map[string]string`
    - Keys and values must be strings.

2. **Metadata**: Include optional contextual information like:
    - Deployment environment
    - Request-specific details (e.g., timestamps, tracing IDs)

   **Type**: `map[string]any`
    - Keys must be strings, and values can be of any type.

When metadata is included, the response structure is:

```json
{
  "data": {},
  "metadata": {}
}
```

If metadata is omitted, the response defaults to:

```json
{
  "data": {}
}
```

### Example Usage

#### Adding Custom Headers and Metadata
To include custom headers and metadata in your response, populate the Headers and MetaData fields of the Response struct in your handler function.

```go
package main

import (
	"time"

	"gofr.dev/pkg/gofr"
	"gofr.dev/pkg/gofr/http/response"
)

func main() {
	app := gofr.New()

	app.GET("/hello", HelloHandler)

	app.Run()
}

func HelloHandler(c *gofr.Context) (any, error) {
	name := c.Param("name")
	if name == "" {
		c.Log("Name parameter is empty, defaulting to 'World'")
		name = "World"
	}

	// Define custom headers (map[string]string)
	headers := map[string]string{
		"X-Custom-Header":  "CustomValue",
		"X-Another-Header": "AnotherValue",
	}

	// Define metadata (map[string]any)
	metaData := map[string]any{
		"environment": "staging",
		"timestamp":   time.Now(),
	}

	// Return response with custom headers and metadata
	return response.Response{
		Data:     map[string]string{"message": "Hello, " + name + "!"},
		Metadata: metaData,
		Headers:  headers,
	}, nil
}
```

### Example Responses
#### Response with Metadata:
When metadata is included, the response contains the metadata field:

```json
{
  "data": {
    "message": "Hello, World!"
  },
  "metadata": {
    "environment": "staging",
    "timestamp": "2024-12-23T12:34:56Z"
  }
}
```

#### Response without Metadata:
If no metadata is provided, the response only includes the data field:

```json
{
  "data": {
    "message": "Hello, World!"
  }
}
```


This functionality offers a convenient, structured way to include additional response information without altering the 
core data payload.