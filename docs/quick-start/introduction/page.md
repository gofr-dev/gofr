---
description: "Build your first GoFr REST API in under 5 minutes. Quick-start tutorial showing the minimal main.go for a Go HTTP service with built-in observability and zero boilerplate."
nextjs:
  metadata:
    title: "Quick Start — Your First GoFr REST API in Go"
    description: "Build your first GoFr REST API in under 5 minutes. The minimal main.go for a Go HTTP service with built-in observability and zero boilerplate."
---

# Hello, GoFr

{% answer %}
GoFr is an opinionated Go framework for production microservices. The fastest path to a running service is: `go mod init`, `go get gofr.dev`, then a `main.go` that calls `gofr.New()` and registers a handler with `app.GET("/greet", handler)`. The framework wires HTTP routing, structured logging, OpenTelemetry traces, Prometheus metrics, datasource clients, and graceful shutdown automatically — no extra setup.
{% /answer %}

GoFr is an opinionated Go framework for production microservices. It bundles HTTP routing, structured logging, OpenTelemetry traces, Prometheus metrics, datasource clients, and graceful shutdown so you can focus on handler logic instead of plumbing. This page gets you from `go mod init` to a running, observable HTTP server in under five minutes.

## Prerequisites

- Go 1.25 or above. Check with `go version`.
- Familiarity with Go syntax — the {% new-tab-link title="Golang Tour" href="https://tour.golang.org/" /%} is a good 30-minute primer if you're new.

## Write your first GoFr API

Let's start by initializing the {% new-tab-link title="go module" href="https://go.dev/ref/mod" /%} by using the following command.

```bash
go mod init github.com/example
```

Add {% new-tab-link title="gofr" href="https://github.com/gofr-dev/gofr" /%} package to the project using the following command.

```bash
go get gofr.dev
```

This code snippet showcases the creation of a simple GoFr application that defines a route and serves a response. 
You can add this code to your main.go file.

```go
package main

import "gofr.dev/pkg/gofr"

func main() {
	// initialize gofr object
	app := gofr.New()

	// register route greet
	app.GET("/greet", func(ctx *gofr.Context) (any, error) {
		return "Hello World!", nil
	})

	// Runs the server, it will listen on the default port 8000.
	// it can be over-ridden through configs
	app.Run()
}
```

Before starting the server, run the following command in your terminal to ensure you have downloaded and synchronized all required dependencies for your project.

`go mod tidy`

Once the dependencies are synchronized, start the GoFr server using the following command:

`go run main.go`

This would start the server at 8000 port, `/greet` endpoint can be accessed from your browser at {% new-tab-link title="http://localhost:8000/greet" href="http://localhost:8000/greet" /%}, you would be able to see the output as following with _Status Code 200_ as per REST Standard.

```json
{"data":"Hello World!"}
```

## Understanding the example

The `hello-world` server involves three essential steps:

1. **Creating GoFr Server:**

   When `gofr.New()` is called, it initializes the framework and handles various setup tasks like initializing logger, metrics, datasources, etc., based on the configs.

   _This single line is a standard part of all GoFr servers._

2. **Attaching a Handler to a Path:**

   In this step, the server is instructed to associate an HTTP request with a specific handler function. This is achieved through `app.GET("/greet", HandlerFunction)`, where _GET /greet_ maps to HandlerFunction. Likewise, `app.POST("/todo", ToDoCreationHandler)` links a _POST_ request to the `/todo` endpoint with _ToDoCreationHandler_.

   **Good To Know**

> In Go, functions are first-class citizens, allowing easy handler definition and reference.
> HTTP Handler functions should follow the `func(ctx *gofr.Context) (any, error)` signature.
> They take a context as input, returning two values: the response data and an error (set to `nil` when there is no error).

GoFr {% new-tab-link  newtab=false title="context" href="/docs/references/context" /%} `ctx *gofr.Context` serves as a wrapper for requests, responses, and dependencies, providing various functionalities.

3. **Starting the server**

   When `app.Run()` is called, it configures, initiates, and runs the HTTP server, middlewares. It manages essential features such as routes for health check endpoints, metrics server, favicon etc. It starts the server on the default port 8000.

## Default ports and endpoints

Out of the box, `app.Run()` opens two listeners (a third is added when you use gRPC). If any of these ports are taken on your machine, GoFr will fail to start — set the matching env var in `configs/.env` to override.

| Server | Default port | Override env var | Endpoints exposed |
|---|---|---|---|
| **HTTP** | `8000` | `HTTP_PORT` | Your routes, plus `/.well-known/health`, `/.well-known/alive`, `/.well-known/swagger`, `/favicon.ico` (and `/.well-known/graphql/ui` if GraphQL is enabled). |
| **Metrics** (Prometheus) | `2121` | `METRICS_PORT` (set to `0` to disable) | `/metrics` (Prometheus exposition format). Scraped by Prometheus / kube-prometheus-stack. |
| **gRPC** | `9000` | `GRPC_PORT` | Your registered gRPC services. Only opened if you call `app.RegisterService(...)`. |

So a fresh `app := gofr.New(); app.Run()` is reachable at:

- `http://localhost:8000/<your-routes>`
- `http://localhost:8000/.well-known/alive` → `200 OK` (use this for K8s liveness probes)
- `http://localhost:8000/.well-known/health` → JSON status of registered datasources (use this for readiness probes)
- `http://localhost:2121/metrics` → Prometheus metrics

All `/.well-known/*` paths are auth-exempt by default, so health probes don't need credentials.

For the full list of configurable env vars, see [GoFr Configuration Options](/docs/references/configs).
