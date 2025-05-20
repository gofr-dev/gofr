# WebRTC Support in GoFr

GoFr now supports built-in WebRTC endpoints for peer-to-peer, real-time streaming (video/audio/data) with observability (metrics, logging, tracing).

## Usage Example

```go
package main

import (
	"gofr.dev/pkg/gofr"
	"gofr.dev/pkg/gofr/webrtc"
	"github.com/pion/webrtc/v4"
)

func main() {
	app := gofr.New()
	cfg := &webrtc.Config{WebRTC: webrtc.Configuration{}}

	app.WebRTC("/webrtc/offer", func(ctx *gofr.Context, pc any) error {
		peerConnection := pc.(*webrtc.PeerConnection)
		peerConnection.OnDataChannel(func(dc *webrtc.DataChannel) {
			dc.OnMessage(func(msg webrtc.DataChannelMessage) {
				ctx.Logger.Infof("Received data channel message: %s", string(msg.Data))
				dc.SendText(string(msg.Data))
			})
		})
		return nil
	}, cfg)

	app.Run()
}
```

## Observability
- **Metrics**: All WebRTC requests, errors, and successes are tracked with custom metrics.
- **Logging**: All negotiation steps and errors are logged.
- **Tracing**: Each handler is wrapped in a trace span for distributed tracing.

## Metrics Exposed
- `webrtc_requests`
- `webrtc_errors` (with stage label)
- `webrtc_success`
- `webrtc_datachannel_messages`

## Requirements
- [pion/webrtc](https://github.com/pion/webrtc) is used under the hood.
- See the [examples/using-web-socket/webrtc.go](../../examples/using-web-socket/webrtc.go) for a full working example.
