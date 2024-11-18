# Setting Custom Response Headers

In GoFr, you can customize HTTP response headers using the `Response` struct, allowing you to add extra information to 
responses sent from your application. This feature can be useful for adding metadata, such as custom headers, security 
policies, or other contextual information, to improve the client-server communication.

## Using the Response Struct

To use custom headers in your handler, create and return a Response object within the handler function. This object 
should contain the response data along with a Headers map for any custom headers you wish to add.

### Example:

Below is an example showing how to use the Response struct in a GoFr handler. In this case, the `HelloHandler` function 
returns a greeting message along with two custom headers: X-Custom-Header and X-Another-Header.

```go
package main

import (
	"gofr.dev/pkg/gofr"
	"gofr.dev/pkg/gofr/http/response"
)

func main() {
	// Create a new application
	a := gofr.New()

	// Add the route
	a.GET("/hello", HelloHandler)

	// Run the application
	a.Run()
}

func HelloHandler(c *gofr.Context) (interface{}, error) {
	name := c.Param("name")
	if name == "" {
		c.Log("Name came empty")
		name = "World"
	}

	headers := map[string]string{
		"X-Custom-Header":  "CustomValue",
		"X-Another-Header": "AnotherValue",
	}

	return response.Response{
		Data:    "Hello World from new Server",
		Headers: headers,
	}, nil
}
```

This functionality offers a convenient, structured way to include additional response information without altering the 
core data payload.