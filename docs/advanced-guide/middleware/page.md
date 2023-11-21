# Middleware

Gofr allows you to create a middleware that take an HTTP handler as input and return an HTTP handler. The middleware function can then modify the request or response before passing it on to the next handler in the chain.

GoFr has some built-in middlewares which are New Relic, LDAP, CORS, OAuth, Tracing which you can enable by adding configurations in `.env` file.

When middleware is added your request flows in the following way.

**Router => Middleware Handler => Request-Handler**

## Adding Custom Middlewares

To use middleware in your Go application, you should use the UseMiddleware method when initializing your app. You can register multiple middlewares, and your incoming requests will flow through all of them before reaching the request handler.

```go
package main

import (
	"gofr.dev/pkg/gofr"
	"net/http"
)

func main() {
	// initialise gofr object
	app := gofr.New()

	// register custom-middlware
	app.Server.UseMiddleware(customMiddleware())

	app.GET("/greet", func(ctx *gofr.Context) (interface{}, error) {

		return "Hello World!", err
	})

	// Starts the server, it will listen on the default port 8000.
	// it can be over-ridden through configs
	app.Start()
}

func customMiddleware() func(handler http.Handler) http.Handler {
	return func(inner http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request){
			// your logic here

			// sends the request to the next middleware/request-handler
			inner.ServeHTTP(w, r)
		})
	}
}
```
