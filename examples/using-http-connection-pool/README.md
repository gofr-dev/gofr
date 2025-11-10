# HTTP Connection Pool Configuration Example

This example demonstrates how to configure HTTP connection pool settings for GoFr HTTP services to optimize performance for high-frequency requests.

## Problem Solved

The default Go HTTP client has `MaxIdleConnsPerHost: 2`, which can cause:
- Connection pool exhaustion errors
- Increased latency (3x slower connection establishment)
- Poor connection reuse

## Configuration Options

- **MaxIdleConns**: Maximum idle connections across all hosts
- **MaxIdleConnsPerHost**: Maximum idle connections per host (critical for performance)
- **IdleConnTimeout**: How long to keep idle connections alive

## Running the Example

```bash
go run main.go
```

Test the endpoint:
```bash
curl http://localhost:8000/posts/1
```

## Benefits

- Eliminates connection pool exhaustion errors
- Improves performance for high-frequency inter-service calls
- Backward compatible with existing code