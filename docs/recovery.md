# Panic Recovery in GoFr

GoFr provides a centralized panic recovery mechanism that ensures your application remains stable and operational even when panics occur in HTTP handlers, goroutines, cron jobs, or CLI commands.

## Overview

The `recovery` package provides a unified approach to handling panics across all execution contexts in GoFr. When a panic is caught, the recovery system:

- **Logs the panic** with a full stack trace using the GoFr logger
- **Emits metrics** to track panic occurrences
- **Creates OpenTelemetry spans** for distributed tracing and debugging

This ensures that panics are not silent failures but are properly instrumented and observable.

## Features

- **Centralized Handling**: All panics are handled consistently across the framework
- **Automatic Logging**: Panics are logged with full stack traces
- **Metrics Integration**: Panic events are tracked via the `panic_total` counter metric
- **Distributed Tracing**: OpenTelemetry spans are created for panic events
- **Type Classification**: Panics are classified by type (string, error, unknown)

## Usage

### HTTP Handlers

HTTP handlers are automatically protected by the recovery middleware. If a panic occurs in an HTTP handler, the middleware will:

1. Catch the panic
2. Log it with stack trace
3. Record a metric
4. Return a 500 Internal Server Error response

```go
app := gofr.New()

// This handler is automatically protected
app.GET("/api/data", func(c *gofr.Context) (any, error) {
    // If this panics, it will be caught and logged
    data := fetchData()
    return data, nil
})
```

### Goroutines

Use `GoSafe` to wrap goroutines with panic recovery:

```go
app := gofr.New()

app.GET("/process", func(c *gofr.Context) (any, error) {
    // Safe goroutine execution
    c.GoSafe(context.Background(), func() {
        // If this panics, it will be caught and logged
        processData()
    })
    
    return map[string]string{"status": "processing"}, nil
})
```

### Cron Jobs

Use `SafeCronFunc` to wrap cron job functions:

```go
app := gofr.New()

// Wrap the cron function with SafeCronFunc
app.AddCronJob("0 * * * *", "hourly-task", app.Recovery.SafeCronFunc(func() {
    // If this panics, it will be caught and logged
    performHourlyTask()
}))
```

### CLI Commands

Use `RunSafeCommand` for CLI command execution:

```go
app := gofr.NewCMD()

app.SubCommand("process", func(c *gofr.Context) (any, error) {
    // Use RunSafeCommand to wrap the operation
    err := c.Recovery.RunSafeCommand(c, func() error {
        // If this panics, it will be caught and logged
        return processCommand()
    })
    
    return nil, err
})
```

## Metrics

The recovery system emits the following metrics:

### `panic_total` (Counter)

Incremented when a panic is caught and recovered. Includes labels:

- `type`: The type of panic value (`string`, `error`, or `unknown`)

Example:
```
panic_total{type="string"} 5
panic_total{type="error"} 2
panic_total{type="unknown"} 1
```

## Tracing

When a panic is recovered, an OpenTelemetry span is created with:

- **Span Name**: `panic_recovery`
- **Status**: Error
- **Attributes**:
  - `panic.value`: The string representation of the panic value
  - `panic.type`: The type of the panic value

This allows you to track panics in your distributed tracing system (Jaeger, Zipkin, etc.).

## Logging

Panics are logged with the following structure:

```json
{
  "level": "ERROR",
  "time": "2024-01-15T10:30:45.123456Z",
  "message": {
    "error": "runtime error: index out of range",
    "stack_trace": "goroutine 42 [running]:\n..."
  },
  "trace_id": "7e5c0e9a58839071d4d006dd1d0f4f3a"
}
```

## Best Practices

1. **Use GoSafe for Long-Running Goroutines**: Always wrap goroutines that perform critical work with `GoSafe` to prevent silent failures.

2. **Monitor Panic Metrics**: Set up alerts on the `panic_total` metric to be notified of panics in production.

3. **Review Panic Logs**: Regularly review panic logs to identify and fix underlying issues.

4. **Test Panic Scenarios**: Include tests that verify panic recovery behavior in your test suite.

5. **Avoid Panicking in Production**: While the recovery system handles panics gracefully, it's better to return errors explicitly when possible.

## Example: Complete Application

```go
package main

import (
    "gofr.dev/pkg/gofr"
)

func main() {
    app := gofr.New()

    // HTTP handler with automatic panic recovery
    app.GET("/api/users/:id", func(c *gofr.Context) (any, error) {
        id := c.Param("id")
        user := fetchUser(id)
        return user, nil
    })

    // Goroutine with explicit panic recovery
    app.GET("/api/process", func(c *gofr.Context) (any, error) {
        c.GoSafe(c, func() {
            processInBackground()
        })
        return map[string]string{"status": "processing"}, nil
    })

    // Cron job with panic recovery
    app.AddCronJob("0 * * * *", "cleanup", app.Recovery.SafeCronFunc(func() {
        cleanupOldData()
    }))

    app.Run()
}

func fetchUser(id string) map[string]string {
    // Implementation
    return map[string]string{"id": id, "name": "John"}
}

func processInBackground() {
    // Implementation
}

func cleanupOldData() {
    // Implementation
}
```

## See Also

- [Error Handling Guide](./advanced-guide/error-handling/page.md)
- [Logging Guide](./advanced-guide/logging/page.md)
- [Metrics Guide](./advanced-guide/metrics/page.md)
- [Tracing Guide](./advanced-guide/tracing/page.md)
