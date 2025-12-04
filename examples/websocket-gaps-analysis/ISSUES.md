# Individual Issues to Create

Based on the POC analysis in [Issue #2578](https://github.com/gofr-dev/gofr/issues/2578), the following individual issues should be created:

## High Priority Issues

### Issue 1: Add Ping/Pong Heartbeat Mechanism for WebSocket Connections

**Title**: `feat: Add ping/pong heartbeat mechanism for WebSocket connections`

**Labels**: `enhancement`, `websocket`, `high-priority`

**Description**:
```markdown
## Problem
Currently, GoFr's WebSocket implementation lacks automatic ping/pong heartbeat functionality to detect dead connections. This means:
- Zombie connections remain in the connection pool
- No way to detect client disconnections gracefully
- Server resources are wasted on dead connections

## Proposed Solution
Add configurable ping/pong handlers with:
- Configurable ping interval (default: 30s)
- Configurable pong wait timeout (default: 60s)
- Automatic connection cleanup on pong timeout
- Hooks for custom ping/pong handlers

## API Design
```go
// Option 1: Global configuration
app.ConfigureWebSocket(gofr.WebSocketConfig{
    PingInterval: 30 * time.Second,
    PongWait:     60 * time.Second,
})

// Option 2: Per-connection configuration  
app.WebSocket("/ws", func(ctx *gofr.Context) (any, error) {
    ctx.SetPingInterval(30 * time.Second)
    // handler logic
})
```

## Implementation Checklist
- [ ] Add ping/pong configuration to WebSocket manager
- [ ] Implement automatic ping sender
- [ ] Implement pong timeout detection
- [ ] Add connection cleanup on timeout
- [ ] Write comprehensive tests
- [ ] Update documentation with examples
- [ ] Add example showing heartbeat usage

## Related
- POC: #2578
```

---

### Issue 2: Add Broadcast Functionality for WebSocket Connections

**Title**: `feat: Add broadcast functionality for WebSocket connections`

**Labels**: `enhancement`, `websocket`, `high-priority`

**Description**:
```markdown
## Problem
There's no built-in way to broadcast messages to multiple WebSocket connections. Developers must:
- Manually iterate through connections
- Implement their own broadcasting logic
- Handle synchronization themselves

## Proposed Solution
Add broadcast methods to the WebSocket Manager:
- Broadcast to all connections
- Broadcast to specific connection groups/rooms
- Filtered broadcasting

## API Design
```go
// Broadcast to all
ws.BroadcastToAll(message)

// Broadcast to room (requires room management)
ws.BroadcastToRoom("room-id", message)

// Broadcast with filter
ws.BroadcastWhere(func(conn *websocket.Connection) bool {
    return conn.Metadata["room"] == "general"
}, message)
```

## Implementation Checklist
- [ ] Add `BroadcastToAll` method to Manager
- [ ] Add room/channel management
- [ ] Add `BroadcastToRoom` method
- [ ] Add filtered broadcast capability
- [ ] Handle concurrent writes safely
- [ ] Write comprehensive tests
- [ ] Update documentation
- [ ] Add chat room example

## Related
- POC: #2578
```

---

### Issue 3: Improve WebSocket Reconnection Strategy

**Title**: `feat: Add exponential backoff and configurable retry strategy for WebSocket reconnection`

**Labels**: `enhancement`, `websocket`, `high-priority`

**Description**:
```markdown
## Problem
Current `AddWSService` reconnection is basic:
- Fixed retry interval
- No exponential backoff
- Retries forever with no limit
- No hooks for connection recovery

## Proposed Solution
Enhance reconnection with:
- Exponential backoff strategy
- Configurable max retry attempts
- Custom retry strategies
- Connection recovery hooks

## API Design
```go
app.AddWSService("service", "ws://example.com/ws", headers, gofr.ReconnectOptions{
    Enabled:        true,
    InitialDelay:   1 * time.Second,
    MaxDelay:       60 * time.Second,
    BackoffFactor:  2.0,
    MaxAttempts:    10, // 0 = infinite
    OnReconnect:    func(attempt int) { ... },
    OnGiveUp:       func(err error) { ... },
})
```

## Implementation Checklist
- [ ] Add ReconnectOptions struct
- [ ] Implement exponential backoff
- [ ] Add max retry limit
- [ ] Add reconnection hooks
- [ ] Add jitter to prevent thundering herd
- [ ] Write comprehensive tests
- [ ] Update documentation
- [ ] Add example demonstrating reconnection

## Related
- POC: #2578
```

---

## Medium Priority Issues

### Issue 4: Complete WebSocket Message Type Support

**Title**: `feat: Export all WebSocket message type constants and improve handling`

**Labels**: `enhancement`, `websocket`, `medium-priority`

**Description**:
```markdown
## Problem
Only `TextMessage` constant is exported. Missing:
- BinaryMessage
- CloseMessage
- PingMessage
- PongMessage

This limits flexibility for handling different message types.

## Proposed Solution
- Export all message type constants
- Add helper methods for different message types
- Improve binary data handling

## Implementation Checklist
- [ ] Export all WebSocket message type constants
- [ ] Add binary message support to `serializeMessage()`
- [ ] Add helper methods (SendBinary, SendText, SendPing, etc.)
- [ ] Write tests for all message types
- [ ] Update documentation
- [ ] Add example showing different message types

## Related
- POC: #2578
```

---

### Issue 5: Add Connection State Management and Metadata

**Title**: `feat: Add WebSocket connection lifecycle management and metadata tracking`

**Labels**: `enhancement`, `websocket`, `medium-priority`

**Description**:
```markdown
## Problem
No way to:
- Track connection state (connecting, open, closing, closed)
- Store connection metadata
- Get connection info (when connected, last activity)
- Gracefully shutdown all connections

## Proposed Solution
Add connection state tracking and metadata management.

## API Design
```go
// Get connection state
state := conn.State() // Connecting, Open, Closing, Closed

// Connection metadata
conn.SetMetadata("user_id", "12345")
userID := conn.GetMetadata("user_id")

// Connection info
info := conn.Info()
// info.ConnectedAt
// info.LastActivity
// info.RemoteAddr

// Graceful shutdown
ws.CloseAllConnections(30 * time.Second)
```

## Implementation Checklist
- [ ] Add connection state enum
- [ ] Add metadata map to Connection
- [ ] Add ConnectionInfo struct
- [ ] Track connected timestamp and last activity
- [ ] Implement graceful shutdown
- [ ] Write comprehensive tests
- [ ] Update documentation

## Related
- POC: #2578
```

---

### Issue 6: Add WebSocket-Specific Error Types

**Title**: `feat: Add structured WebSocket-specific error types`

**Labels**: `enhancement`, `websocket`, `medium-priority`

**Description**:
```markdown
## Problem
Limited error handling:
- No WebSocket-specific error types
- Limited error context
- Hard to distinguish error types

## Proposed Solution
Define WebSocket-specific error types with proper error wrapping.

## API Design
```go
var (
    ErrConnectionClosed     = errors.New("connection closed")
    ErrInvalidMessageType   = errors.New("invalid message type")
    ErrBroadcastFailed      = errors.New("broadcast failed")
    ErrConnectionTimeout    = errors.New("connection timeout")
    ErrInvalidPayload       = errors.New("invalid payload")
)

type WebSocketError struct {
    Op     string // operation
    ConnID string // connection ID
    Err    error  // underlying error
}
```

## Implementation Checklist
- [ ] Define error types
- [ ] Implement error wrapping
- [ ] Add context to error messages
- [ ] Update error handling throughout
- [ ] Write tests
- [ ] Update documentation

## Related
- POC: #2578
```

---

### Issue 7: Add WebSocket Middleware Support

**Title**: `feat: Add middleware chain for WebSocket operations`

**Labels**: `enhancement`, `websocket`, `medium-priority`

**Description**:
```markdown
## Problem
No WebSocket-specific middleware for:
- Message validation
- Rate limiting
- Authentication per message
- Message transformation

## Proposed Solution
Add WebSocket middleware interface similar to HTTP middleware.

## API Design
```go
type WSMiddleware func(WSHandler) WSHandler

app.WebSocket("/ws", handler,
    gofr.WSRateLimit(10, time.Second),
    gofr.WSValidateMessage(schema),
    gofr.WSAuthenticate(),
)
```

## Implementation Checklist
- [ ] Define WebSocket middleware interface
- [ ] Implement middleware chain
- [ ] Add built-in middleware (rate limit, auth, validation)
- [ ] Write tests
- [ ] Update documentation
- [ ] Add middleware examples

## Related
- POC: #2578
```

---

### Issue 8: Add Connection Limits and Rate Limiting

**Title**: `feat: Add connection limits and message rate limiting for WebSocket`

**Labels**: `enhancement`, `websocket`, `medium-priority`

**Description**:
```markdown
## Problem
No built-in protection against:
- Too many concurrent connections
- Message flooding
- Resource exhaustion

## Proposed Solution
Add configurable limits for connections and messages.

## API Design
```go
app.ConfigureWebSocket(gofr.WebSocketConfig{
    MaxConnections:     1000,
    MaxConnectionsPerIP: 10,
    MessageRateLimit:   100, // messages per second
    ConnectionThrottle: 10 * time.Millisecond,
})
```

## Implementation Checklist
- [ ] Add max connections limit
- [ ] Add per-IP connection limit
- [ ] Add message rate limiting
- [ ] Add connection throttling
- [ ] Write tests
- [ ] Update documentation

## Related
- POC: #2578
```

---

### Issue 9: Add WebSocket Metrics and Monitoring

**Title**: `feat: Add metrics and monitoring for WebSocket connections`

**Labels**: `enhancement`, `websocket`, `observability`, `medium-priority`

**Description**:
```markdown
## Problem
No visibility into:
- Active connection count
- Message throughput
- Connection errors
- Performance metrics

## Proposed Solution
Add metrics that integrate with GoFr's observability stack.

## Metrics to Track
- Active connections (gauge)
- Total connections (counter)
- Messages sent/received (counter)
- Connection errors (counter)
- Message latency (histogram)
- Connection duration (histogram)

## Implementation Checklist
- [ ] Add metrics collection
- [ ] Integrate with GoFr observability
- [ ] Add metrics endpoint
- [ ] Write tests
- [ ] Update documentation with metrics guide
- [ ] Add monitoring example

## Related
- POC: #2578
```

---

## Low Priority Issues

### Issue 10: Add WebSocket Compression Support

**Title**: `feat: Add per-message compression support for WebSocket`

**Labels**: `enhancement`, `websocket`, `low-priority`

**Description**:
```markdown
## Problem
No compression configuration may impact performance for large payloads.

## Proposed Solution
Add compression options to WSUpgrader.

## Implementation Checklist
- [ ] Add compression options
- [ ] Document compression trade-offs
- [ ] Add benchmarks
- [ ] Update documentation

## Related
- POC: #2578
```

---

### Issue 11: Add WebSocket Subprotocol Support

**Title**: `feat: Add WebSocket subprotocol negotiation support`

**Labels**: `enhancement`, `websocket`, `low-priority`

**Description**:
```markdown
## Problem
No way to negotiate or handle WebSocket subprotocols.

## Proposed Solution
Add subprotocol negotiation to upgrader configuration.

## Implementation Checklist
- [ ] Add subprotocol negotiation
- [ ] Allow developers to specify supported subprotocols
- [ ] Write tests
- [ ] Update documentation

## Related
- POC: #2578
```

---

### Issue 12: Enhance WebSocket Testing Utilities

**Title**: `feat: Add comprehensive WebSocket testing utilities`

**Labels**: `enhancement`, `websocket`, `testing`, `low-priority`

**Description**:
```markdown
## Problem
Limited testing support:
- No test server utilities
- No WebSocket testing helpers
- Basic mock interfaces

## Proposed Solution
Add comprehensive testing utilities.

## Implementation Checklist
- [ ] Add WebSocket test server
- [ ] Add testing helper functions
- [ ] Enhance mock interfaces
- [ ] Add example test patterns
- [ ] Update documentation

## Related
- POC: #2578
```

---

### Issue 13: Enhance WebSocket Documentation

**Title**: `docs: Add comprehensive WebSocket documentation and examples`

**Labels**: `documentation`, `websocket`, `medium-priority`

**Description**:
```markdown
## Problem
Limited documentation for:
- Advanced use cases
- Best practices
- Troubleshooting
- Production patterns

## Proposed Solution
Create comprehensive documentation.

## Documentation Needed
- [ ] Advanced examples (chat, notifications, etc.)
- [ ] Best practices guide
- [ ] Troubleshooting guide
- [ ] Production deployment patterns
- [ ] Performance tuning guide
- [ ] Security considerations

## Related
- POC: #2578
```

---

## Summary

**Total Issues**: 13  
**High Priority**: 3  
**Medium Priority**: 7  
**Low Priority**: 3

These issues can be created independently and assigned to different contributors according to [CONTRIBUTING.md](../../CONTRIBUTING.md) guidelines.
