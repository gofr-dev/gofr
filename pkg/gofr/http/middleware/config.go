package middleware

import (
	"strconv"
	"strings"

	"golang.org/x/text/cases"
	"golang.org/x/text/language"

	"gofr.dev/pkg/gofr/config"
	"gofr.dev/pkg/gofr/service"
)

type Config struct {
	CorsHeaders map[string]string
	LogProbes   LogProbes

	// MetricsCardinalityLimit is the meter provider's effective per-instrument
	// datapoint ceiling, resolved the way the provider resolves it:
	// METRICS_CARDINALITY_LIMIT, then OTEL_GO_X_CARDINALITY_LIMIT, then the SDK
	// default. Zero or negative means unlimited. The Metrics middleware sizes its
	// caller-controlled label budget from it; see WithCardinalityLimit.
	MetricsCardinalityLimit int
}

// defaultMetricsCardinalityLimit is the OTel SDK's per-instrument default, used
// when neither METRICS_CARDINALITY_LIMIT nor OTEL_GO_X_CARDINALITY_LIMIT is set.
const defaultMetricsCardinalityLimit = 2000

const (
	metricsCardinalityLimitKey = "METRICS_CARDINALITY_LIMIT"
	otelCardinalityLimitKey    = "OTEL_GO_X_CARDINALITY_LIMIT"
)

type LogProbes struct {
	Disabled bool
	Paths    []string
}

func GetConfigs(c config.Config) Config {
	middlewareConfigs := Config{
		CorsHeaders: make(map[string]string),
	}

	allowedCORSHeaders := []string{
		"ACCESS_CONTROL_ALLOW_ORIGIN",
		"ACCESS_CONTROL_ALLOW_METHODS",
		"ACCESS_CONTROL_ALLOW_HEADERS",
		"ACCESS_CONTROL_ALLOW_CREDENTIALS",
		"ACCESS_CONTROL_EXPOSE_HEADERS",
		"ACCESS_CONTROL_MAX_AGE",
	}

	for _, v := range allowedCORSHeaders {
		if val := c.Get(v); val != "" {
			middlewareConfigs.CorsHeaders[convertHeaderNames(v)] = val
		}
	}

	// Config values for Log Probes
	logDisableProbes := c.GetOrDefault("LOG_DISABLE_PROBES", "false")
	middlewareConfigs.LogProbes.Paths = []string{service.HealthPath, service.AlivePath}

	// Convert the string value to a boolean
	value, err := strconv.ParseBool(logDisableProbes)
	if err == nil {
		middlewareConfigs.LogProbes.Disabled = value
	}

	middlewareConfigs.MetricsCardinalityLimit = metricsCardinalityLimit(c)

	return middlewareConfigs
}

// metricsCardinalityLimit resolves the provider ceiling with the same precedence
// the provider applies: METRICS_CARDINALITY_LIMIT (container/metrics_exporter.go,
// which also warns on an invalid value), then OTEL_GO_X_CARDINALITY_LIMIT (read
// by the SDK itself), then the SDK default. A value that does not parse is
// skipped, as both of those skip it.
func metricsCardinalityLimit(c config.Config) int {
	for _, key := range []string{metricsCardinalityLimitKey, otelCardinalityLimitKey} {
		if n, err := strconv.Atoi(strings.TrimSpace(c.Get(key))); err == nil {
			return n
		}
	}

	return defaultMetricsCardinalityLimit
}

func convertHeaderNames(header string) string {
	words := strings.Split(header, "_")
	titleCaser := cases.Title(language.Und)

	for i, v := range words {
		words[i] = titleCaser.String(strings.ToLower(v))
	}

	return strings.Join(words, "-")
}
