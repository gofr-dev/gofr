
## WebRTC Example

This example demonstrates how to use pion/webrtc for peer-to-peer, real-time streaming (data channel) in GoFr, with built-in observability (metrics, logging, tracing).

### Usage

1. Start the server:

   ```bash
   go run main.go webrtc.go
   ```

2. Send a POST request to `/webrtc/offer` with a WebRTC SDP offer (see pion/webrtc examples for client code).

3. The server responds with an SDP answer. Data channel messages are echoed and all events are logged, metered, and traced.

### Observability
- Metrics: `webrtc_requests`, `webrtc_errors`, `webrtc_datachannel_messages`, `webrtc_success`
- Logging: All negotiation and data events
- Tracing: Each handler invocation is traced
