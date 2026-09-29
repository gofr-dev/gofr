package gofr

import (
	"errors"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"

	"gofr.dev/pkg/gofr/container"
	"gofr.dev/pkg/gofr/logging"
)

var errTelemetryTransport = errors.New("telemetry transport unavailable")

func TestApp_sendTelemetry_TransportError(t *testing.T) {
	tests := []struct {
		desc    string
		isStart bool
		expURL  string
	}{
		{desc: "start ping", isStart: true, expURL: gofrHost + startServerPing},
		{desc: "shutdown ping", isStart: false, expURL: gofrHost + shutServerPing},
	}

	for _, tc := range tests {
		t.Run(tc.desc, func(t *testing.T) {
			transport := &mockRoundTripper{mockError: errTelemetryTransport}

			a := &App{container: &container.Container{Logger: logging.NewMockLogger(logging.FATAL)}}

			// A failed ping must be swallowed: telemetry never affects the app.
			assert.NotPanics(t, func() { a.sendTelemetry(&http.Client{Transport: transport}, tc.isStart) })

			assert.Equal(t, tc.expURL, transport.lastRequest.URL.String())
			assert.Equal(t, "close", transport.lastRequest.Header.Get("Connection"))
		})
	}
}
