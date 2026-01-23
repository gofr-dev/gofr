package exporters

import (
	"context"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
)

type mockLogger struct {
	logs []string
}

func (l *mockLogger) Infof(format string, args ...any) {
	for _, arg := range args {
		format = strings.Replace(format, "%s", arg.(string), 1)
	}

	l.logs = append(l.logs, format)
}

func TestLog(t *testing.T) {
	logger := &mockLogger{}
	appName := "test-app"
	appVersion := "v1.0.0"

	meter, flush := Log(appName, appVersion, logger)
	assert.NotNil(t, meter)
	assert.NotNil(t, flush)

	counter, _ := meter.Int64Counter("test_counter", metric.WithDescription("test counter"))
	counter.Add(context.Background(), 1, metric.WithAttributes(attribute.String("label", "value")))

	err := flush(context.Background())
	require.NoError(t, err)

	assert.NotEmpty(t, logger.logs, "No metrics logged")

	found := false

	for _, log := range logger.logs {
		if strings.Contains(log, "[GOFR_METRICS]") &&
			strings.Contains(log, "test_counter") &&
			strings.Contains(log, "label") &&
			strings.Contains(log, "value") {
			found = true
			break
		}
	}

	assert.True(t, found, "Expected JSON metric log not found")
}
