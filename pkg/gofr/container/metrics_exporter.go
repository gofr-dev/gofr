package container

import (
	"strconv"
	"strings"
	"time"

	"gofr.dev/pkg/gofr/config"
	"gofr.dev/pkg/gofr/metrics/exporters"
)

const (
	defaultMetricsExportInterval = 30
	authorizationHeader          = "Authorization"
)

// metricsExporterConfig resolves the METRICS_* configuration into an
// exporters.Config. METRICS_PORT is handled separately (it controls the pull
// server), so it is intentionally absent here.
func metricsExporterConfig(conf config.Config, appName, appVersion string) exporters.Config {
	interval := defaultMetricsExportInterval

	if v := conf.Get("METRICS_EXPORT_INTERVAL"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			interval = n
		}
	}

	insecure := true

	if v := conf.Get("METRICS_INSECURE"); v != "" {
		if b, err := strconv.ParseBool(v); err == nil {
			insecure = b
		}
	}

	return exporters.Config{
		AppName:     appName,
		AppVersion:  appVersion,
		Exporter:    strings.TrimSpace(conf.Get("METRICS_EXPORTER")),
		Endpoint:    strings.TrimSpace(conf.Get("METRICS_URL")),
		Protocol:    conf.GetOrDefault("METRICS_PROTOCOL", "grpc"),
		Interval:    time.Duration(interval) * time.Second,
		Temporality: conf.GetOrDefault("METRICS_TEMPORALITY", "cumulative"),
		Headers:     metricsHeaders(conf),
		Insecure:    insecure,
	}
}

// metricsHeaders builds export headers from METRICS_HEADERS (OTEL "k=v,k=v"
// format) or, if unset, from METRICS_AUTH_KEY as an Authorization header.
func metricsHeaders(conf config.Config) map[string]string {
	if h := conf.Get("METRICS_HEADERS"); h != "" {
		return parseMetricsHeaders(h)
	}

	if key := conf.Get("METRICS_AUTH_KEY"); key != "" {
		return map[string]string{authorizationHeader: key}
	}

	return nil
}

func parseMetricsHeaders(headerStr string) map[string]string {
	const keyValueParts = 2

	headers := make(map[string]string)

	for _, pair := range strings.Split(headerStr, ",") {
		kv := strings.SplitN(strings.TrimSpace(pair), "=", keyValueParts)
		if len(kv) != keyValueParts {
			continue
		}

		key := strings.TrimSpace(kv[0])
		value := strings.TrimSpace(kv[1])

		if key != "" && value != "" {
			headers[key] = value
		}
	}

	return headers
}
