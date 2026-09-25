package exporters

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel/sdk/metric/metricdata"
)

func Test_httpEndpointWithSignalPath(t *testing.T) {
	tests := []struct {
		name     string
		endpoint string
		want     string
	}{
		{name: "path-less URL gains the signal path", endpoint: "http://host:4318", want: "http://host:4318/v1/metrics"},
		{name: "https path-less URL gains it too", endpoint: "https://host:4318", want: "https://host:4318/v1/metrics"},
		{name: "explicit signal path is preserved", endpoint: "http://host:4318/v1/metrics", want: "http://host:4318/v1/metrics"},
		{name: "explicit custom path is preserved", endpoint: "http://host:4318/otlp/v1/metrics", want: "http://host:4318/otlp/v1/metrics"},
		{name: "explicit root path is left alone", endpoint: "http://host:4318/", want: "http://host:4318/"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.Equal(t, tt.want, httpEndpointWithSignalPath(tt.endpoint))
		})
	}
}

// Test_buildOTLPExporter_HTTPPathLessURLKeepsSignalPath is the end-to-end guard the
// existing table missed: it drives a real export at a live server and asserts the
// request lands on /v1/metrics for a path-less METRICS_URL, which is what regressed
// when otlpmetrichttp moved past otel v1.45.
func Test_buildOTLPExporter_HTTPPathLessURLKeepsSignalPath(t *testing.T) {
	tests := []struct {
		name     string
		path     string // path suffix on the endpoint, "" = path-less
		wantPath string
	}{
		{name: "path-less", path: "", wantPath: otlpMetricsSignalPath},
		{name: "explicit signal path", path: otlpMetricsSignalPath, wantPath: otlpMetricsSignalPath},
		{name: "custom path", path: "/otlp/v1/metrics", wantPath: "/otlp/v1/metrics"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := make(chan string, 1)

			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				select {
				case got <- r.URL.Path:
				default:
				}

				w.WriteHeader(http.StatusOK)
			}))
			defer srv.Close()

			exp, err := buildOTLPExporter(context.Background(), &Config{
				Endpoint: srv.URL + tt.path,
				Protocol: protocolHTTP,
				Insecure: true,
				Interval: time.Second,
			})
			require.NoError(t, err)

			_ = exp.Export(context.Background(), &metricdata.ResourceMetrics{})

			select {
			case p := <-got:
				require.Equal(t, tt.wantPath, p)
			case <-time.After(3 * time.Second):
				t.Fatal("no request reached the collector")
			}
		})
	}
}
