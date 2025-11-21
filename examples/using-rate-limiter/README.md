# HTTP Rate Limiter Example

This example demonstrates how to use the HTTP rate limiter middleware in GoFr to protect your API from abuse and ensure fair resource distribution.

## Features

- **Token Bucket Algorithm**: Smooth rate limiting with configurable burst
- **Per-IP Rate Limiting**: Each client IP gets its own rate limit
- **Automatic Health Check Exemption**: `/.well-known/alive` and `/.well-known/health` are not rate limited
- **Prometheus Metrics**: Track rate limit violations
- **429 Status Code**: Standard HTTP response when limit exceeded

## Configuration

```go
rateLimiterConfig := middleware.RateLimiterConfig{
    RequestsPerSecond: 5,    // Average requests per second
    Burst:             10,   // Maximum burst size
    PerIP:             true, // Enable per-IP limiting (false for global)
}

app.UseMiddleware(middleware.RateLimiter(rateLimiterConfig, app.Metrics()))
```

### Parameters

- **RequestsPerSecond**: Average number of requests allowed per second
- **Burst**: Maximum number of requests that can be made in a burst (allows temporary spikes)
- **PerIP**: 
  - `true`: Each IP address gets its own rate limit (recommended)
  - `false`: Global rate limit shared across all clients

## Running the Example

```bash
go run main.go
```

The server will start on `http://localhost:8000`

## Testing Rate Limiting

### Test 1: Normal Requests (Within Limit)
```bash
# Send a few requests - should succeed
curl http://localhost:8000/limited
curl http://localhost:8000/limited
curl http://localhost:8000/limited
```

**Expected**: All requests return `200 OK`

### Test 2: Exceed Rate Limit
```bash
# Send many rapid requests
for i in {1..15}; do
  curl -w "\nStatus: %{http_code}\n" http://localhost:8000/limited
  echo "---"
done
```

**Expected**: 
- First 10 requests succeed (burst capacity)
- Subsequent requests return `429 Too Many Requests`

### Test 3: Health Endpoints (Always Accessible)
```bash
# Health endpoints are never rate limited
for i in {1..20}; do
  curl http://localhost:8000/.well-known/alive
done
```

**Expected**: All requests succeed with `200 OK`

### Test 4: Token Refill
```bash
# Exhaust rate limit
for i in {1..12}; do curl http://localhost:8000/limited; done

# Wait 1 second for tokens to refill
sleep 1

# Try again - should succeed
curl http://localhost:8000/limited
```

**Expected**: Request after waiting succeeds

### Test 5: Per-IP Isolation
If you have access to multiple IPs or can use a proxy:

```bash
# Terminal 1 (IP1)
curl http://localhost:8000/limited

# Terminal 2 (IP2 via proxy)
curl -x http://proxy:8080 http://localhost:8000/limited
```

**Expected**: Each IP has independent rate limits

## Monitoring

View Prometheus metrics at `http://localhost:2121/metrics`:

```bash
curl http://localhost:2121/metrics | grep rate_limit
```

**Metrics:**
- `app_http_rate_limit_exceeded_total`: Counter of rejected requests

## Use Cases

### Production API Protection
```go
rateLimiterConfig := middleware.RateLimiterConfig{
    RequestsPerSecond: 100,
    Burst:             200,
    PerIP:             true,
}
```

### Development/Staging
```go
rateLimiterConfig := middleware.RateLimiterConfig{
    RequestsPerSecond: 10,
    Burst:             20,
    PerIP:             true,
}
```

### Global Rate Limit (All Clients)
```go
rateLimiterConfig := middleware.RateLimiterConfig{
    RequestsPerSecond: 1000,
    Burst:             2000,
    PerIP:             false, // Shared limit
}
```

## Notes

- Rate limiter uses `golang.org/x/time/rate` for efficient token bucket implementation
- Stale per-IP limiters are cleaned up automatically every 5 minutes
- IP extraction order: `X-Forwarded-For` → `X-Real-IP` → `RemoteAddr`
- Works seamlessly with other middleware (auth, logging, metrics)
