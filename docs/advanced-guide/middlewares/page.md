# Middleware in GoFr

Middleware allows you intercepting and manipulating HTTP requests and responses flowing through your application's
router. Middlewares can perform tasks such as authentication, authorization, caching etc. before
or after the request reaches your application's handler.

## Adding Custom Middleware in GoFr

By adding custom middleware to your GoFr application, user can easily extend its functionality and implement 
cross-cutting concerns in a modular and reusable way.
User can use the `UseMiddleware` method on your GoFr application instance to register your custom middleware.

### Example:

```go
import (
    "net/http"

    "gofr.dev/pkg/gofr"
)

// Define your custom middleware function
func customMiddleware() gofr.Middleware {
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

