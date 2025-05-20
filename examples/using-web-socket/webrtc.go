package main

import (
	"fmt"
	"net/http"
	"os"
	"time"

	"gofr.dev/pkg/gofr"
	"github.com/pion/webrtc/v4"
)

// WebRTCHandler demonstrates a simple WebRTC offer/answer exchange and data channel echo with observability.
func WebRTCHandler(ctx *gofr.Context) (any, error) {
	ctx.Logger.Infof("WebRTC handler invoked")
	ctx.Metrics.IncrementCounter("webrtc_requests", nil)
	ctx.Tracer.StartSpan("webrtc_handler")
	defer ctx.Tracer.EndSpan()

	// Parse SDP offer from request
	var offer webrtc.SessionDescription
	if err := ctx.Bind(&offer); err != nil {
		ctx.Logger.Errorf("Failed to bind SDP offer: %v", err)
		ctx.Metrics.IncrementCounter("webrtc_errors", map[string]string{"stage": "bind"})
		return nil, err
	}

	// Prepare WebRTC API
	peerConnection, err := webrtc.NewPeerConnection(webrtc.Configuration{})
	if err != nil {
		ctx.Logger.Errorf("Failed to create PeerConnection: %v", err)
		ctx.Metrics.IncrementCounter("webrtc_errors", map[string]string{"stage": "peerconnection"})
		return nil, err
	}
	defer peerConnection.Close()

	// Data channel echo for demonstration
	peerConnection.OnDataChannel(func(dc *webrtc.DataChannel) {
		dc.OnMessage(func(msg webrtc.DataChannelMessage) {
			ctx.Logger.Infof("Received data channel message: %s", string(msg.Data))
			ctx.Metrics.IncrementCounter("webrtc_datachannel_messages", nil)
			dc.SendText(string(msg.Data))
		})
	})

	// Set remote description
	if err := peerConnection.SetRemoteDescription(offer); err != nil {
		ctx.Logger.Errorf("Failed to set remote description: %v", err)
		ctx.Metrics.IncrementCounter("webrtc_errors", map[string]string{"stage": "set_remote"})
		return nil, err
	}

	// Create answer
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

	ctx.Logger.Infof("WebRTC negotiation complete")
	ctx.Metrics.IncrementCounter("webrtc_success", nil)

	// Wait for ICE gathering
	gatherComplete := webrtc.GatheringCompletePromise(peerConnection)
	<-gatherComplete

	return peerConnection.LocalDescription(), nil
}

func main() {
	app := gofr.New()
	app.POST("/webrtc/offer", WebRTCHandler)
	app.Run()
}
