package exporters

import (
	"net/http"
	"net/http/httptest"
	"runtime"
	"testing"

	"github.com/stretchr/testify/assert"

	"gofr.dev/pkg/gofr/version"
)

func TestSendFrameworkStartupTelemetry_Disabled(t *testing.T) {
	// Set environment variable to disable telemetry
	t.Setenv("GOFR_TELEMETRY_DISABLED", "true")

	requestMade := false

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		requestMade = true

		w.WriteHeader(http.StatusOK)
	}))

	defer server.Close()

	SendFrameworkStartupTelemetry("test-app", "1.0.0")

	if requestMade {
		t.Error("Expected no telemetry when disabled, but request was made")
	}
}

func TestCreateTelemetryResource(t *testing.T) {
	appName := "test-app"
	appVersion := "1.0.0"

	resource := createTelemetryResource(appName, appVersion)

	if resource == nil {
		t.Fatal("Expected resource to be created, got nil")
	}

	// Verify attributes exist
	attrs := resource.Attributes()

	expectedAttrs := map[string]string{
		"service.name":      appName,
		"service.version":   appVersion,
		"framework":         "gofr",
		"framework_version": version.Framework,
		"go_version":        runtime.Version(),
		"os":                runtime.GOOS,
		"arch":              runtime.GOARCH,
	}

	// Convert slice to map for easier lookup
	foundAttrs := make(map[string]string)
	for _, attr := range attrs {
		foundAttrs[string(attr.Key)] = attr.Value.AsString()
	}

	for key, expected := range expectedAttrs {
		actual, exists := foundAttrs[key]

		assert.True(t, exists, "Expected attribute %s to exist: %v", key)
		assert.Equal(t, expected, actual, "Expected %s = %s, got %s", key, expected, actual)
	}
}
