// Package gcp registers a keyless OTLP trace exporter that pushes spans
// directly to Google Cloud's Telemetry (OTLP) API — the ingestion front door to
// Cloud Trace — using Application Default Credentials.
//
// Blank-import it and set TRACE_EXPORTER=gcp:
//
//	import _ "gofr.dev/pkg/gofr/traces/exporters/gcp"
//
// On Cloud Run this authenticates via the attached service account, with no key
// file and no Collector sidecar. Grant that service account
// roles/telemetry.tracesWriter (or the broader roles/telemetry.writer) on the
// project receiving the spans.
//
// roles/cloudtrace.agent is NOT sufficient: it authorizes the older Cloud Trace
// API (cloudtrace.googleapis.com), which this exporter never calls.
//
// TRACER_URL is optional and defaults to telemetry.googleapis.com:443. Set it to
// a regional endpoint (telemetry.<region>.rep.googleapis.com:443) when data
// residency requires one.
package gcp

import (
	"context"
	"fmt"
	"os"
	"strings"
	"sync"

	gcpdetect "go.opentelemetry.io/contrib/detectors/gcp"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"gofr.dev/pkg/gofr/traces/exporters"
	"golang.org/x/oauth2/google"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/oauth"
)

const (
	exporterName       = "gcp"
	defaultEndpoint    = "telemetry.googleapis.com:443"
	cloudPlatformScope = "https://www.googleapis.com/auth/cloud-platform"

	// projectIDKey is the resource attribute Google's OTLP ingest reads to route
	// spans to a destination project.
	projectIDKey = "gcp.project_id"

	// projectEnv is the Google-standard fallback for the destination project,
	// read directly because it is a Google SDK convention rather than a GoFr
	// config (the metrics exporter reads OTEL_RESOURCE_ATTRIBUTES the same way).
	projectEnv = "GOOGLE_CLOUD_PROJECT"

	// quotaProjectHeader is resolved by Google from the service account itself and
	// documented as not settable this way.
	quotaProjectHeader = "x-goog-user-project"
)

//nolint:gochecknoinits // self-registration on blank import is the intended usage.
func init() {
	exporters.Register(exporterName, buildExporter)
	exporters.RegisterResourceDetector(exporterName, &cachingDetector{})
}

// adc resolves Application Default Credentials at most once per instance. The
// detector and the builder both need them — the detector for the destination
// project, the builder for the token source — and there is no reason to make two
// round trips to the metadata server for an answer that cannot differ.
type adc struct {
	once  sync.Once
	creds *google.Credentials
	err   error
}

func (a *adc) get(ctx context.Context) (*google.Credentials, error) {
	a.once.Do(func() {
		a.creds, a.err = google.FindDefaultCredentials(ctx, cloudPlatformScope)
	})

	return a.creds, a.err
}

//nolint:gochecknoglobals // one ADC resolution per process, shared by the detector and the builder.
var defaultADC adc

// cachingDetector runs the GCP metadata detector at most once per process and
// adds gcp.project_id, which the platform detector does not supply.
//
// Detect is where the project attribute has to be set: Google's migration
// guidance indicates the destination project for spans is taken from the
// resource unless something else supplies it, and that behavior is not
// documented precisely enough for a plain Go gRPC exporter to rely on either
// way. Setting it from the ambient credentials is correct whether or not it is
// strictly required, which is the point — it cannot be wrong, and its absence
// could be.
type cachingDetector struct {
	once sync.Once
	res  *resource.Resource
	err  error

	// adc is overridable so tests can drive project resolution without touching
	// the process-wide credentials.
	adc *adc
}

// Detect satisfies resource.Detector. Off Google Cloud the underlying detector
// fails; the error is returned so the SDK can report a partial resource, and the
// exporter still starts — a span dropped by the backend is strictly better than
// an application that will not boot.
func (d *cachingDetector) Detect(ctx context.Context) (*resource.Resource, error) {
	d.once.Do(func() {
		d.res, d.err = gcpdetect.NewDetector().Detect(ctx)

		if project := d.projectID(ctx); project != "" {
			d.res = withProjectID(d.res, project)
		}
	})

	return d.res, d.err
}

// projectID resolves the destination project from the ambient credentials,
// falling back to GOOGLE_CLOUD_PROJECT. An authorized_user credential carries no
// project, so the fallback is the normal path for local development rather than
// an edge case.
func (d *cachingDetector) projectID(ctx context.Context) string {
	source := d.adc
	if source == nil {
		source = &defaultADC
	}

	if creds, err := source.get(ctx); err == nil && creds.ProjectID != "" {
		return creds.ProjectID
	}

	return strings.TrimSpace(os.Getenv(projectEnv))
}

// withProjectID merges gcp.project_id into res, tolerating a nil res (which is
// what the platform detector returns off Google Cloud).
func withProjectID(res *resource.Resource, project string) *resource.Resource {
	attrs := resource.NewSchemaless(attribute.String(projectIDKey, project))

	if res == nil {
		return attrs
	}

	merged, err := resource.Merge(res, attrs)
	if err != nil {
		return attrs
	}

	return merged
}

// buildExporter builds an OTLP gRPC span exporter authenticated with Google
// Application Default Credentials (ADC). On Cloud Run this uses the attached
// service account via the metadata server — no key file. The token source
// refreshes automatically, which Google's direct OTLP ingest requires (~1h token
// lifetime) and which no static TRACER_HEADERS value can do.
func buildExporter(ctx context.Context, cfg *exporters.Config, logger exporters.Logger) (sdktrace.SpanExporter, error) {
	// The destination project comes off the resource, and the resource is already
	// built by the time a builder runs, so this is the last point at which an
	// operator can be told before spans quietly go to the wrong place or nowhere.
	warnMissingProject(cfg, logger)

	creds, err := defaultADC.get(ctx)
	if err != nil {
		return nil, fmt.Errorf("gcp traces: resolving application default credentials: %w", err)
	}

	endpoint := cfg.Endpoint
	if endpoint == "" {
		endpoint = defaultEndpoint
	}

	opts := []otlptracegrpc.Option{
		otlptracegrpc.WithEndpoint(endpoint),
		otlptracegrpc.WithTLSCredentials(credentials.NewTLS(nil)),
		otlptracegrpc.WithDialOption(
			grpc.WithPerRPCCredentials(oauth.TokenSource{TokenSource: creds.TokenSource}),
		),
	}

	if len(cfg.Headers) > 0 {
		if _, ok := cfg.Headers[quotaProjectHeader]; ok {
			logger.Warnf("gcp traces: ignore-listed header %s is set; "+
				"set GOOGLE_CLOUD_QUOTA_PROJECT instead, or attach a service account", quotaProjectHeader)
		}

		opts = append(opts, otlptracegrpc.WithHeaders(cfg.Headers))
	}

	exporter, err := otlptracegrpc.New(ctx, opts...)
	if err != nil {
		return nil, fmt.Errorf("gcp traces: creating OTLP exporter: %w", err)
	}

	logger.Infof("exporting traces to Google Cloud at %s via keyless ADC", endpoint)

	return exporter, nil
}

// warnMissingProject reports that no destination project could be resolved.
// cfg.Resource is the resource the TracerProvider will actually export, so this
// sees what the detector produced rather than guessing again.
func warnMissingProject(cfg *exporters.Config, logger exporters.Logger) {
	if resolvedProject(cfg.Resource) != "" {
		return
	}

	logger.Warnf("gcp traces: no %s could be resolved from the ambient credentials and this host is not "+
		"on Google Cloud; spans may not reach a project. Set %s=<project-id>", projectIDKey, projectEnv)
}

func resolvedProject(res *resource.Resource) string {
	if res == nil {
		return ""
	}

	for _, kv := range res.Attributes() {
		if string(kv.Key) == projectIDKey {
			return kv.Value.AsString()
		}
	}

	return ""
}
