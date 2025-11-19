# Panic Recovery Example

This example demonstrates how GoFr's centralized panic recovery mechanism works across different execution contexts.

## Overview

GoFr automatically catches and handles panics in:
- HTTP handlers
- Goroutines (when using `GoSafe`)
- Cron jobs
- CLI commands

When a panic is caught, it is:
- Logged with a full stack trace
- Tracked via metrics
- Recorded in OpenTelemetry traces

## Running the Example

```bash
go run main.go
```

The server will start on the default HTTP port (8080).

## Endpoints

### `/panic` - HTTP Handler Panic
Triggers a panic in an HTTP handler. The panic is caught and logged, and a 500 error is returned.

```bash
curl http://localhost:8080/panic
```

Expected response:
```json
{
  "code": 500,
  "status": "ERROR",
  "message": "Some unexpected error has occurred"
}
```

Check the logs to see the panic details and stack trace.

### `/async-panic` - Goroutine Panic
Starts a goroutine using `GoSafe` that panics. The panic is caught and logged, but the HTTP response is returned immediately.

```bash
curl http://localhost:8080/async-panic
```

Expected response:
```json
{
  "data": {
    "status": "processing"
  }
}
```

Check the logs to see the goroutine panic details.

### `/status` - Application Status
Returns statistics about panics and recovered goroutines.

```bash
curl http://localhost:8080/status
```

Expected response:
```json
{
  "data": {
    "panic_count": 2,
    "recover_count": 1,
    "timestamp": 1705315200
  }
}
```

## Observability

### Metrics

The recovery system emits the `panic_total` metric with labels for panic type:

```
panic_total{type="string"} 5
panic_total{type="error"} 2
```

### Logs

Panics are logged with full stack traces:

```json
{
  "level": "ERROR",
  "time": "2024-01-15T10:30:45.123456Z",
  "message": {
    "error": "intentional panic #1",
    "stack_trace": "goroutine 42 [running]:\nmain.main.func1(...)"
  }
}
```

### Traces

OpenTelemetry spans are created for panic events with attributes:
- `panic.value`: The panic message
- `panic.type`: The type of panic (string, error, unknown)

## Testing Panic Recovery

Try the following scenarios:

1. **Multiple HTTP panics**: Call `/panic` multiple times to verify the server remains operational
2. **Concurrent goroutine panics**: Call `/async-panic` multiple times to test concurrent panic recovery
3. **Monitor metrics**: Check the `panic_total` metric to see panic counts by type
4. **Review logs**: Examine application logs to see panic details and stack traces

## Key Takeaways

- Panics are automatically caught in HTTP handlers
- Use `GoSafe` to safely execute goroutines with panic recovery
- All panics are logged with full context and stack traces
- Metrics and traces help with observability and debugging
- The application remains stable and operational after panics

## See Also

- [Recovery Documentation](../../docs/recovery.md)
- [Error Handling Guide](../../docs/advanced-guide/error-handling/page.md)
- [Metrics Guide](../../docs/advanced-guide/metrics/page.md)
