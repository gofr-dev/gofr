---
name: gofr-rest-api
description: Build a REST API in Go with GoFr — routing, handlers, request binding, path and query parameters, typed errors, and auto-generated CRUD handlers. Use when writing a new Go HTTP service or adding endpoints to an existing GoFr application.
license: Apache-2.0
---

# Building a REST API with GoFr

GoFr is an opinionated Go framework. `gofr.New()` returns an app with HTTP
routing, structured logging, OpenTelemetry tracing, Prometheus metrics, health
endpoints, and graceful shutdown already wired. Do not add those yourself.

## The one handler signature

Every handler in GoFr — HTTP, gRPC, GraphQL, WebSocket, cron, CLI — has the same
shape:

```go
func(c *gofr.Context) (any, error)
```

Return a value and GoFr wraps it as `{"data": ...}`. Return an error and GoFr
maps it to the right status code. Do not write to a `http.ResponseWriter`.

## Minimal service

```go
package main

import "gofr.dev/pkg/gofr"

func main() {
    app := gofr.New()

    app.GET("/users/{id}", getUser)
    app.POST("/users", createUser)

    app.Run()
}

func getUser(c *gofr.Context) (any, error) {
    id := c.PathParam("id")

    var u User
    err := c.SQL.QueryRowContext(c, "SELECT id, name FROM users WHERE id = $1", id).
        Scan(&u.ID, &u.Name)
    if err != nil {
        return nil, err
    }

    return u, nil
}
```

## Rules that trip people up

- **Path params use `{name}`, not `:name`.** `app.GET("/users/{id}", h)`.
- `c.PathParam("id")` for path, `c.Param("q")` for query, `c.Bind(&v)` for body.
- **Pass `c` itself as the context** to every downstream call — `QueryContext(c, …)`,
  not `context.Background()`. That is what propagates the trace and the deadline.
- Configuration comes from `configs/.env` via `c.Config.Get("KEY")`. Never
  hardcode a port, host, or credential.
- Health is already served at `/.well-known/health` and `/.well-known/alive`.
  Do not add `/healthz`.
- Middleware is standard `net/http`: `func(http.Handler) http.Handler`, registered
  with `app.UseMiddleware(...)`.

## Errors

Return GoFr's typed errors so the status code is correct. A bare
`fmt.Errorf(...)` becomes a 500 regardless of what actually went wrong.

```go
import "gofr.dev/pkg/gofr/http"

return nil, http.ErrorEntityNotFound{Name: "user", Value: id}   // 404
return nil, http.ErrorInvalidParam{Params: []string{"email"}}   // 400
```

## Zero-boilerplate CRUD

For a table that maps cleanly to a struct, register the whole CRUD surface at
once instead of writing five handlers:

```go
type User struct {
    ID   int    `json:"id"`
    Name string `json:"name"`
}

if err := app.AddRESTHandlers(&User{}); err != nil {
    app.Logger().Errorf("could not register CRUD handlers: %v", err)
}
```

## Read next

- <https://gofr.dev/docs/quick-start/introduction.md>
- <https://gofr.dev/docs/quick-start/add-rest-handlers.md>
- <https://gofr.dev/docs/advanced-guide/gofr-errors.md>
- <https://gofr.dev/docs/advanced-guide/middlewares.md>
