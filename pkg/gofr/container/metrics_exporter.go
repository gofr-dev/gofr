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
	defaultTemporality           = "cumulative"
)

// metricsExporterConfig resolves the METRICS_* configuration into an
// exporters.Config. METRICS_PORT is handled separately (it controls the pull
// server), so it is intentionally absent here.
//
// METRICS_INSECURE defaults to false (secure by default, matching the OTel
// SDK): a schemeless METRICS_URL (host:port) connects over TLS unless
// METRICS_INSECURE=true is set explicitly. A METRICS_URL with an explicit
// http:// or https:// scheme derives security from the scheme instead,
// regardless of METRICS_INSECURE (see exporters/otlp.go).
//
// Interval and temporality fall back to the OpenTelemetry standard env vars
// (OTEL_METRIC_EXPORT_INTERVAL, OTEL_EXPORTER_OTLP_METRICS_TEMPORALITY_PREFERENCE)
// when the GoFr-native METRICS_* vars are unset, so a user already standardized
// on OTEL_* gets honored. GoFr config wins when both are set. These are read via
// conf.Get (GoFr surfaces OS env through its config layer), not by letting the
// SDK read env directly.
func metricsExporterConfig(conf config.Config, appName, appVersion string, logger exporters.Logger) exporters.Config {
	insecure := false

	if v := conf.Get("METRICS_INSECURE"); v != "" {
		if b, err := strconv.ParseBool(v); err == nil {
			insecure = b
		}
	}

	headers := metricsHeaders(conf)

	if insecure && len(headers) > 0 && logger != nil {
		logger.Warnf("METRICS_INSECURE=true with credentials/headers configured: " +
			"headers (including auth credentials) will be sent in plaintext")
	}

	return exporters.Config{
		AppName:     appName,
		AppVersion:  appVersion,
		Exporter:    strings.TrimSpace(conf.Get("METRICS_EXPORTER")),
		Endpoint:    strings.TrimSpace(conf.Get("METRICS_URL")),
		Protocol:    conf.GetOrDefault("METRICS_PROTOCOL", "grpc"),
		Interval:    metricsExportInterval(conf, logger),
		Temporality: metricsTemporality(conf),
		Headers:     headers,
		Insecure:    insecure,
	}
}

// metricsExportInterval resolves the push interval, preferring the GoFr-native
// METRICS_EXPORT_INTERVAL (seconds), then the OpenTelemetry standard
// OTEL_METRIC_EXPORT_INTERVAL (milliseconds, per spec), then the default.
func metricsExportInterval(conf config.Config, logger exporters.Logger) time.Duration {
	if v := conf.Get("METRICS_EXPORT_INTERVAL"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			return time.Duration(n) * time.Second
		} else if logger != nil {
			logger.Warnf("invalid METRICS_EXPORT_INTERVAL %q; "+
				"falling back to OTEL_METRIC_EXPORT_INTERVAL or default %ds", v, defaultMetricsExportInterval)
		}
	}

	if v := conf.Get("OTEL_METRIC_EXPORT_INTERVAL"); v != "" {
		if ms, err := strconv.Atoi(v); err == nil && ms > 0 {
			return time.Duration(ms) * time.Millisecond
		} else if logger != nil {
			logger.Warnf("invalid OTEL_METRIC_EXPORT_INTERVAL %q; defaulting to %ds", v, defaultMetricsExportInterval)
		}
	}

	return defaultMetricsExportInterval * time.Second
}

// metricsTemporality resolves the temporality preference, preferring the
// GoFr-native METRICS_TEMPORALITY, then the OpenTelemetry standard
// OTEL_EXPORTER_OTLP_METRICS_TEMPORALITY_PREFERENCE (same cumulative/delta/
// lowmemory vocabulary), then cumulative. Validation/warning on an unrecognized
// value happens in exporters/otlp.go.
func metricsTemporality(conf config.Config) string {
	if v := strings.TrimSpace(conf.Get("METRICS_TEMPORALITY")); v != "" {
		return v
	}

	if v := strings.TrimSpace(conf.Get("OTEL_EXPORTER_OTLP_METRICS_TEMPORALITY_PREFERENCE")); v != "" {
		return v
	}

	return defaultTemporality
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
