package exporters

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"os"
	"runtime"
	"time"

	"github.com/google/uuid"

	"gofr.dev/pkg/gofr/version"
)

const (
	defaultTelemetryEndpoint = "https://gofr.dev/telemetry/v1/metrics"
	defaultAppName           = "gofr-app"
	requestTimeout           = 10 * time.Second
)

// TelemetryData represents the JSON telemetry payload.
type TelemetryData struct {
	Timestamp        string `json:"timestamp"`
	EventID          string `json:"event_id"`
	Source           string `json:"source"`
	ServiceName      string `json:"service_name,omitempty"`
	ServiceVersion   string `json:"service_version,omitempty"`
	RawDataSize      int    `json:"raw_data_size"`
	FrameworkVersion string `json:"framework_version,omitempty"`
	GoVersion        string `json:"go_version,omitempty"`
	OS               string `json:"os,omitempty"`
	Architecture     string `json:"architecture,omitempty"`
	StartupTime      string `json:"startup_time,omitempty"`
}

// SendFrameworkStartupTelemetry sends telemetry data.
func SendFrameworkStartupTelemetry(appName, appVersion string) {
	if os.Getenv("GOFR_TELEMETRY_DISABLED") == "true" {
		return
	}

	go sendTelemetryData(appName, appVersion)
}

func sendTelemetryData(appName, appVersion string) {
	if appName == "" {
		appName = defaultAppName
	}

	if appVersion == "" {
		appVersion = "unknown"
	}

	now := time.Now().UTC()

	data := TelemetryData{
		Timestamp:        now.Format(time.RFC3339),
		EventID:          uuid.New().String(),
		Source:           "gofr-framework",
		ServiceName:      appName,
		ServiceVersion:   appVersion,
		RawDataSize:      0,
		FrameworkVersion: version.Framework,
		GoVersion:        runtime.Version(),
		OS:               runtime.GOOS,
		Architecture:     runtime.GOARCH,
		StartupTime:      now.Format(time.RFC3339),
	}

	sendToEndpoint(&data, defaultTelemetryEndpoint)
}

func sendToEndpoint(data *TelemetryData, endpoint string) {
	jsonData, err := json.Marshal(data)
	if err != nil {
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), requestTimeout)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewBuffer(jsonData))
	if err != nil {
		return
	}

	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 10 * time.Second}

	resp, err := client.Do(req)
	if err != nil {
		return
	}

	resp.Body.Close()
}
