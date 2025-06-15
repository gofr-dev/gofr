package webrtc

import (
	"testing"
	"github.com/pion/webrtc/v4"
	"gofr.dev/pkg/gofr"
)

func TestRegister(t *testing.T) {
	app := gofr.New()
	cfg := &Config{WebRTC: webrtc.Configuration{}}
	Register(app, "/test-webrtc", func(ctx *gofr.Context, pc *webrtc.PeerConnection) error {
		return nil
	}, cfg)
}
