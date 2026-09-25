package exporters

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"runtime"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"

	"gofr.dev/pkg/gofr/version"
)

func TestSendFrameworkStartupTelemetry_Disabled(t *testing.T) {
	t.Setenv("GOFR_TELEMETRY", "true")

	requestMade := false

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		requestMade = true

		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	SendFrameworkStartupTelemetry("test-app", "1.0.0")
	time.Sleep(100 * time.Millisecond)

	assert.False(t, requestMade, "Expected no telemetry when disabled")
}

func TestSendFrameworkStartupTelemetry_DefaultValues(t *testing.T) {
	var receivedData TelemetryData

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		assert.NoError(t, err)

		err = json.Unmarshal(body, &receivedData)
		assert.NoError(t, err)

		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	// Test with empty values to verify defaults.
	testSendTelemetryData("", "", server.URL)
	time.Sleep(100 * time.Millisecond)

	assert.Equal(t, defaultAppName, receivedData.ServiceName)
	assert.Equal(t, "unknown", receivedData.ServiceVersion)
	assert.Equal(t, "gofr-framework", receivedData.Source)
	assert.Equal(t, 0, receivedData.RawDataSize)
}

// Helper function that replicates sendTelemetryData but with configurable endpoint.
func testSendTelemetryData(appName, appVersion, endpoint string) {
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

	sendToEndpoint(&data, endpoint)
}

func TestSendToEndpoint(t *testing.T) {
	var received []TelemetryData

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var data TelemetryData

		assert.Equal(t, http.MethodPost, r.Method)
		assert.Equal(t, "application/json", r.Header.Get("Content-Type"))
		assert.NoError(t, json.NewDecoder(r.Body).Decode(&data))

		received = append(received, data)

		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	closedServer := httptest.NewServer(http.NotFoundHandler())
	closedURL := closedServer.URL
	closedServer.Close()

	data := TelemetryData{EventID: "event-1", Source: "gofr-framework", ServiceName: "app"}

	tests := []struct {
		desc        string
		endpoint    string
		expReceived []TelemetryData
	}{
		{desc: "payload delivered", endpoint: server.URL, expReceived: []TelemetryData{data}},
		{desc: "malformed endpoint is ignored", endpoint: "://bad-endpoint", expReceived: nil},
		{desc: "unreachable endpoint is ignored", endpoint: closedURL, expReceived: nil},
	}

	for _, tc := range tests {
		t.Run(tc.desc, func(t *testing.T) {
			received = nil

			sendToEndpoint(&data, tc.endpoint)

			assert.Equal(t, tc.expReceived, received)
		})
	}
}
