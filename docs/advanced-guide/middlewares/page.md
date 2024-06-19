# Middleware in GoFr

Middleware allows you intercepting and manipulating HTTP requests and responses flowing through your application's
router. Middlewares can perform tasks such as authentication, authorization, caching etc. before
or after the request reaches your application's handler.

## CORS Middleware in GoFr
GoFr includes built-in CORS (Cross-Origin Resource Sharing) middleware to handle CORS-related headers. 
This middleware allows you to control access to your API from different origins. It automatically adds the necessary
headers to responses, allowing or restricting cross-origin requests. User can also override the default response headers
sent by GoFr by providing the suitable CORS configs.

The CORS middleware provides the following overridable configs:

- `ACCESS_CONTROL_ALLOW_ORIGIN`: Set the allowed origin(s) for cross-origin requests. By default, it allows all origins (*).
- `ACCESS_CONTROL_ALLOW_HEADERS`: Define the allowed request headers (e.g., Authorization, Content-Type).
- `ACCESS_CONTROL_ALLOW_CREDENTIALS`: Set to true to allow credentials (cookies, HTTP authentication) in requests.
- `ACCESS_CONTROL_EXPOSE_HEADERS`: Specify additional headers exposed to the client.
- `ACCESS_CONTROL_MAX_AGE`: Set the maximum time (in seconds) for preflight request caching.

> Note: GoFr automatically interprets the registered route methods and based on that sets the value of `ACCESS_CONTROL_ALLOW_METHODS`


## Adding Custom Middleware in GoFr

By adding custom middleware to your GoFr application, user can easily extend its functionality and implement 
cross-cutting concerns in a modular and reusable way.
User can use the `UseMiddleware` method on your GoFr application instance to register your custom middleware.

### Example:

```go
import (
    "net/http"

    gofrHTTP "gofr.dev/pkg/gofr/http"
)

// Define your custom middleware function
func customMiddleware() gofrHTTP.Middleware {
    return func(inner http.Handler) http.Handler {
        return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
            // Your custom logic here
            // For example, logging, authentication, etc.
            
            // Call the next handler in the chain
            inner.ServeHTTP(w, r)
        })
    }
}

func main() {
    // Create a new instance of your GoFr application
    app := gofr.New()

    // Add your custom middleware to the application
    app.UseMiddleware(customMiddleware())

    // Define your application routes and handlers
    // ...

    // Run your GoFr application
    app.Run()
}
```

