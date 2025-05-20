package webrtc

import (
	"context"
	"github.com/pion/webrtc/v4"
	"gofr.dev/pkg/gofr"
)

// Handler is the function signature for WebRTC handlers in GoFr.
type Handler func(ctx *gofr.Context, pc *webrtc.PeerConnection) error

// Config holds configuration for WebRTC peer connections.
type Config struct {
	WebRTC webrtc.Configuration
}

// Register sets up a WebRTC endpoint with observability hooks.
func Register(app *gofr.App, path string, handler Handler, cfg *Config) {
	app.POST(path, func(ctx *gofr.Context) (any, error) {
		ctx.Logger.Infof("WebRTC handler invoked")
		ctx.Metrics.IncrementCounter("webrtc_requests", nil)
		ctx.Tracer.StartSpan("webrtc_handler")
		defer ctx.Tracer.EndSpan()

		var offer webrtc.SessionDescription
		if err := ctx.Bind(&offer); err != nil {
			ctx.Logger.Errorf("Failed to bind SDP offer: %v", err)
			ctx.Metrics.IncrementCounter("webrtc_errors", map[string]string{"stage": "bind"})
			return nil, err
		}

		peerConnection, err := webrtc.NewPeerConnection(cfg.WebRTC)
		if err != nil {
			ctx.Logger.Errorf("Failed to create PeerConnection: %v", err)
			ctx.Metrics.IncrementCounter("webrtc_errors", map[string]string{"stage": "peerconnection"})
			return nil, err
		}
		defer peerConnection.Close()

		if err := peerConnection.SetRemoteDescription(offer); err != nil {
			ctx.Logger.Errorf("Failed to set remote description: %v", err)
			ctx.Metrics.IncrementCounter("webrtc_errors", map[string]string{"stage": "set_remote"})
			return nil, err
		}

		if err := handler(ctx, peerConnection); err != nil {
			ctx.Logger.Errorf("WebRTC handler error: %v", err)
			ctx.Metrics.IncrementCounter("webrtc_errors", map[string]string{"stage": "handler"})
			return nil, err
		}

		answer, err := peerConnection.CreateAnswer(nil)
		if err != nil {
			ctx.Logger.Errorf("Failed to create answer: %v", err)
			ctx.Metrics.IncrementCounter("webrtc_errors", map[string]string{"stage": "create_answer"})
			return nil, err
		}
		if err := peerConnection.SetLocalDescription(answer); err != nil {
			ctx.Logger.Errorf("Failed to set local description: %v", err)
			ctx.Metrics.IncrementCounter("webrtc_errors", map[string]string{"stage": "set_local"})
			return nil, err
		}

		gatherComplete := webrtc.GatheringCompletePromise(peerConnection)
		<-gatherComplete

		ctx.Logger.Infof("WebRTC negotiation complete")
		ctx.Metrics.IncrementCounter("webrtc_success", nil)
		return peerConnection.LocalDescription(), nil
	})
}
