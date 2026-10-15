package exporters

import (
	"context"
	"errors"
	"strings"

	metricSdk "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/metric/metricdata"
)

// errEmptyOTLPEndpoint is returned when METRICS_EXPORTER=otlp is selected but
// METRICS_URL is empty. Unlike gcp (which legitimately defaults its endpoint
// to the Google Cloud telemetry API), otlp has no sane default target: an
// empty endpoint would otherwise dial lazily and fail silently on every
// export interval. Returning an error here lets pushReader's degrade-to-
// prometheus path log it visibly instead.
var errEmptyOTLPEndpoint = errors.New("METRICS_URL is required for METRICS_EXPORTER=otlp")

//nolint:gochecknoinits // self-registration of the built-in OTLP exporter is the intended pattern.
func init() {
	Register("otlp", buildOTLPReader)
}

// buildOTLPReader builds a periodic push reader that exports metrics over OTLP
// gRPC (default) or HTTP, honoring the endpoint, headers, insecure flag,
// temporality preference and interval from cfg.
func buildOTLPReader(ctx context.Context, cfg *Config, logger Logger) (metricSdk.Reader, error) {
	if strings.TrimSpace(cfg.Endpoint) == "" {
		return nil, errEmptyOTLPEndpoint
	}

	if p := strings.TrimSpace(cfg.Protocol); p != "" && !strings.EqualFold(p, protocolGRPC) && !strings.EqualFold(p, protocolHTTP) {
		logger.Warnf("unrecognized METRICS_PROTOCOL %q; defaulting to %q", cfg.Protocol, protocolGRPC)
	}

	if t := strings.TrimSpace(cfg.Temporality); t != "" && !strings.EqualFold(t, temporalityCumulative) &&
		!strings.EqualFold(t, temporalityDelta) && !strings.EqualFold(t, temporalityLowMemory) {
		logger.Warnf("unrecognized METRICS_TEMPORALITY %q; defaulting to %q", cfg.Temporality, temporalityCumulative)
	}

	exporter, err := buildOTLPExporter(ctx, cfg)
	if err != nil {
		return nil, err
	}

	logger.Infof("exporting metrics via OTLP %s to %s every %s (temporality=%s)",
		protocol(cfg.Protocol), cfg.Endpoint, cfg.Interval, temporalityName(cfg.Temporality))

	return metricSdk.NewPeriodicReader(exporter, metricSdk.WithInterval(cfg.Interval)), nil
}

// temporalitySelector maps the METRICS_TEMPORALITY preference to an OTLP
// TemporalitySelector, following the OpenTelemetry standard preferences:
// delta -> counters/histograms/observable-counters are delta, rest cumulative;
// lowmemory -> only sync counters/histograms are delta; default cumulative.
func temporalitySelector(pref string) metricSdk.TemporalitySelector {
	switch strings.ToLower(pref) {
	case temporalityDelta:
		return func(k metricSdk.InstrumentKind) metricdata.Temporality {
			switch k { //nolint:exhaustive // default returns cumulative for all remaining instrument kinds
			case metricSdk.InstrumentKindCounter,
				metricSdk.InstrumentKindHistogram,
				metricSdk.InstrumentKindObservableCounter:
				return metricdata.DeltaTemporality
			default:
				return metricdata.CumulativeTemporality
			}
		}
	case temporalityLowMemory:
		return func(k metricSdk.InstrumentKind) metricdata.Temporality {
			switch k { //nolint:exhaustive // default returns cumulative for all remaining instrument kinds
			case metricSdk.InstrumentKindCounter, metricSdk.InstrumentKindHistogram:
				return metricdata.DeltaTemporality
			default:
				return metricdata.CumulativeTemporality
			}
		}
	default:
		return metricSdk.DefaultTemporalitySelector
	}
}

func protocol(p string) string {
	if strings.EqualFold(p, protocolHTTP) {
		return protocolHTTP
	}

	return protocolGRPC
}

func temporalityName(t string) string {
	switch strings.ToLower(t) {
	case temporalityDelta:
		return temporalityDelta
	case temporalityLowMemory:
		return temporalityLowMemory
	default:
		return temporalityCumulative
	}
}
