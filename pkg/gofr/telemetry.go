package gofr

import (
	"context"
	"fmt"
	"net/http"
	"testing"
)

func (a *App) hasTelemetry() bool {
	// Check if running inside a test (Go 1.21+)
	if testing.Testing() {
		return false
	}

	return a.Config.GetOrDefault("GOFR_TELEMETRY", defaultTelemetry) == "true"

}

func (a *App) sendTelemetry(client *http.Client, isStart bool) {
	url := fmt.Sprint(gofrHost, shutServerPing)

	if isStart {
		url = fmt.Sprint(gofrHost, startServerPing)

		a.container.Info("GoFr records the number of active servers. Set GOFR_TELEMETRY=false in configs to disable it.")
	}

	ctx, cancel := context.WithTimeout(context.Background(), pingTimeout)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, http.NoBody)
	if err != nil {
		return
	}

	req.Header.Set("Connection", "close")

	resp, err := client.Do(req)
	if err != nil {
		return
	}

	resp.Body.Close()
}
