package exporters

import (
	"context"
	"strings"

	"go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetricgrpc"
	"go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetrichttp"
	metricSdk "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/metric/metricdata"
)

//nolint:gochecknoinits // self-registration of the built-in OTLP exporter is the intended pattern.
func init() {
	Register("otlp", buildOTLPReader)
}

// buildOTLPReader builds a periodic push reader that exports metrics over OTLP
// gRPC (default) or HTTP, honoring the endpoint, headers, insecure flag,
// temporality preference and interval from cfg.
func buildOTLPReader(ctx context.Context, cfg *Config, logger Logger) (metricSdk.Reader, error) {
	exporter, err := buildOTLPExporter(ctx, cfg)
	if err != nil {
		return nil, err
	}

	logger.Infof("exporting metrics via OTLP %s to %s every %s (temporality=%s)",
		protocol(cfg.Protocol), cfg.Endpoint, cfg.Interval, temporalityName(cfg.Temporality))

	return metricSdk.NewPeriodicReader(exporter, metricSdk.WithInterval(cfg.Interval)), nil
}

func buildOTLPExporter(ctx context.Context, cfg *Config) (metricSdk.Exporter, error) {
	selector := temporalitySelector(cfg.Temporality)
	hasScheme := strings.HasPrefix(cfg.Endpoint, "http://") || strings.HasPrefix(cfg.Endpoint, "https://")

	if strings.EqualFold(cfg.Protocol, protocolHTTP) {
		opts := []otlpmetrichttp.Option{otlpmetrichttp.WithTemporalitySelector(selector)}

		if hasScheme {
			opts = append(opts, otlpmetrichttp.WithEndpointURL(cfg.Endpoint))
		} else {
			opts = append(opts, otlpmetrichttp.WithEndpoint(cfg.Endpoint))
		}

		if cfg.Insecure {
			opts = append(opts, otlpmetrichttp.WithInsecure())
		}

		if len(cfg.Headers) > 0 {
			opts = append(opts, otlpmetrichttp.WithHeaders(cfg.Headers))
		}

		return otlpmetrichttp.New(ctx, opts...)
	}

	opts := []otlpmetricgrpc.Option{otlpmetricgrpc.WithTemporalitySelector(selector)}

	if hasScheme {
		opts = append(opts, otlpmetricgrpc.WithEndpointURL(cfg.Endpoint))
	} else {
		opts = append(opts, otlpmetricgrpc.WithEndpoint(cfg.Endpoint))
	}

	if cfg.Insecure {
		opts = append(opts, otlpmetricgrpc.WithInsecure())
	}

	if len(cfg.Headers) > 0 {
		opts = append(opts, otlpmetricgrpc.WithHeaders(cfg.Headers))
	}

	return otlpmetricgrpc.New(ctx, opts...)
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
