package exporters

import (
	"errors"
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"

	"gofr.dev/pkg/gofr/logging"
	"gofr.dev/pkg/gofr/testutil"
)

func Test_redactExporterName(t *testing.T) {
	tests := []struct {
		name     string
		value    string
		expected string
	}{
		{name: "typo of a real exporter is echoed", value: "otpl", expected: "otpl"},
		{name: "mixed case and separators are echoed", value: "Open_Telemetry-x", expected: "Open_Telemetry-x"},
		{name: "empty is replaced", value: "", expected: "REDACTED"},
		{name: "longer than 16 is replaced", value: "abcdefghijklmnopq", expected: "REDACTED"},
		{name: "digits look like a pasted secret", value: "ab12cd34", expected: "REDACTED"},
		{name: "URL pasted into the wrong variable", value: "https://x", expected: "REDACTED"},
		{name: "control character is replaced", value: "otlp\n", expected: "REDACTED"},
		{name: "non-ASCII letter is replaced", value: "ötlp", expected: "REDACTED"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.Equal(t, tt.expected, redactExporterName(tt.value))
		})
	}
}

func TestRedactURL(t *testing.T) {
	tests := []struct {
		name     string
		raw      string
		expected string
	}{
		{name: "schemeless host:port is unchanged", raw: "collector:4317", expected: "collector:4317"},
		{name: "plain https URL is unchanged", raw: "https://collector:4317", expected: "https://collector:4317"},
		{name: "path is kept", raw: "http://localhost:2005/api/v2/spans", expected: "http://localhost:2005/api/v2/spans"},
		{name: "empty is unchanged", raw: "", expected: ""},
		{name: "userinfo with password", raw: "https://user:s3cret@collector:4317", expected: "https://REDACTED@collector:4317"},
		{name: "userinfo token only", raw: "https://t0ken@collector:4317", expected: "https://REDACTED@collector:4317"},
		{name: "query credentials", raw: "https://zipkin/api/v2/spans?api-key=s3cret", expected: "https://zipkin/api/v2/spans?REDACTED"},
		{name: "fragment is dropped", raw: "https://collector:4317#s3cret", expected: "https://collector:4317"},
		{name: "schemeless userinfo", raw: "user:s3cret@collector:4317", expected: "REDACTED@collector:4317"},
		{name: "unparsable with userinfo", raw: "https://user:s3cret@collector:43%17", expected: "REDACTED@collector:43%17"},
		{name: "schemeless query credentials", raw: "localhost:9411/api/v2/spans?api-key=s3cret",
			expected: "localhost:9411/api/v2/spans?REDACTED"},
		{name: "schemeless userinfo and query", raw: "user:s3cret@collector:4317/p?k=s3cret", expected: "REDACTED@collector:4317/p?REDACTED"},
		{name: "schemeless password containing '?'", raw: "user:s3?cret@collector:4317", expected: "REDACTED@collector:4317"},
		{name: "schemeless '@' inside query value", raw: "collector:4317/p?k=s3cret@x", expected: "REDACTED@x"},
		{name: "schemeless fragment is dropped", raw: "collector:4317#s3cret", expected: "collector:4317"},
		{name: "newline cannot forge a log line", raw: "collector:4317\n{\"level\":\"INFO\"}",
			expected: `collector:4317\x0a{"level":"INFO"}`},
		{name: "carriage return and tab are escaped", raw: "host\r:4317\t", expected: `host\x0d:4317\x09`},
		{name: "C1 control is escaped", raw: "host" + string(rune(0x85)) + ":4317", expected: `host\x85:4317`},
		{name: "control char with userinfo is escaped after redaction", raw: "https://user:s3cret@collector\n:4317",
			expected: `REDACTED@collector\x0a:4317`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.Equal(t, tt.expected, RedactURL(tt.raw))
		})
	}
}

// errProbe stands in for the third-party constructor error whose message
// redactEndpointInError rewrites.
var errProbe = errors.New("dial failed")

func Test_redactEndpointInError(t *testing.T) {
	const endpoint = "https://user:s3cret@collector:4317"

	tests := []struct {
		name     string
		err      error
		endpoint string
		expected string
	}{
		{name: "nil error stays nil", err: nil, endpoint: endpoint, expected: ""},
		{name: "endpoint quoted verbatim is redacted", err: fmt.Errorf("%q: %w", endpoint, errProbe),
			endpoint: endpoint, expected: `"https://REDACTED@collector:4317": dial failed`},
		{name: "endpoint unquoted is redacted", err: fmt.Errorf("%s: %w", endpoint, errProbe), endpoint: endpoint,
			expected: "https://REDACTED@collector:4317: dial failed"},
		{name: "an error not naming the endpoint is untouched", err: errProbe,
			endpoint: endpoint, expected: "dial failed"},
		{name: "no endpoint to match", err: errProbe, endpoint: "", expected: "dial failed"},
		// url.Error renders the URL with %q, so a control character reaches the
		// message as the two bytes `\n` and never matches the raw endpoint.
		{name: "percent-quoted control character is redacted", err: fmt.Errorf("parse %q: %w", "h://user:s3cret@c\n:4317", errProbe),
			endpoint: "h://user:s3cret@c\n:4317", expected: `parse "REDACTED@c\x0a:4317": dial failed`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := redactEndpointInError(tt.err, tt.endpoint)

			if tt.expected == "" {
				require.NoError(t, got)
				return
			}

			require.EqualError(t, got, tt.expected)
			require.NotContains(t, got.Error(), "s3cret")
		})
	}
}

// Test_builders_doNotLogCredentials drives every built-in builder with a
// credential-bearing endpoint and asserts that neither the logs nor the returned
// error carry it. It covers the log lines that name an endpoint or the configured
// exporter name — the ones a resolved TRACER_URL flows into.
func Test_builders_doNotLogCredentials(t *testing.T) {
	const secret = "s3cret-value"

	tests := []struct {
		name      string
		cfg       *Config
		expected  string
		needsOTLP bool
	}{
		{name: "otlp userinfo", cfg: &Config{Exporter: "otlp", Endpoint: "https://user:" + secret + "@localhost:4317"},
			expected: "Exporting traces to otlp at https://REDACTED@localhost:4317", needsOTLP: true},
		{name: "jaeger logs the matched name, not the configured one",
			cfg:      &Config{Exporter: "  JaEgEr  ", Endpoint: "http://user:" + secret + "@localhost:4317"},
			expected: "Exporting traces to jaeger at http://REDACTED@localhost:4317", needsOTLP: true},
		{name: "ignored TRACER_INSECURE names the endpoint",
			cfg:      &Config{Exporter: "otlp", Endpoint: "http://user:" + secret + "@localhost:4317", Insecure: true, InsecureSet: true},
			expected: `TRACER_INSECURE is ignored for TRACER_URL="http://REDACTED@localhost:4317"`, needsOTLP: true},
		{name: "plaintext-with-headers warning names the endpoint",
			cfg: &Config{Exporter: "otlp", Endpoint: "localhost:4317/p?api-key=" + secret, Insecure: true,
				Headers: map[string]string{"Authorization": "Bearer " + secret}},
			expected: "traces are exported to localhost:4317/p?REDACTED over plaintext", needsOTLP: true},
		{name: "zipkin query key", cfg: &Config{Exporter: "zipkin", Endpoint: "http://localhost:2005/api/v2/spans?api-key=" + secret},
			expected: "Exporting traces to zipkin at http://localhost:2005/api/v2/spans?REDACTED"},
		{name: "zipkin schemeless query key", cfg: &Config{Exporter: "zipkin", Endpoint: "localhost:9411/api/v2/spans?api-key=" + secret},
			expected: "Exporting traces to zipkin at localhost:9411/api/v2/spans?REDACTED"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.needsOTLP && !otlpTraceLinked {
				t.Skip("built with -tags gofr_nootlp; the OTLP builder that emits this line is not linked")
			}

			build, ok := lookup(exporterName(tt.cfg.Exporter))
			require.True(t, ok, "no builder registered for %q", tt.cfg.Exporter)

			var err error

			out := testutil.StdoutOutputForFunc(func() {
				_, err = build(t.Context(), tt.cfg, logging.NewMockLogger(logging.DEBUG))
			})

			if err != nil {
				require.NotContains(t, err.Error(), secret)
			}

			require.Contains(t, out, tt.expected)
			require.NotContains(t, out, secret)
		})
	}
}

// Test_zipkin_errorDoesNotLeakCredentials pins the one builder whose constructor
// echoes the endpoint back inside its error: zipkin.New reports an unparsable
// collector URL as `invalid collector URL "<raw>"`.
func Test_zipkin_errorDoesNotLeakCredentials(t *testing.T) {
	const secret = "s3cret-value"

	cfg := &Config{Exporter: exporterZipkin, Endpoint: "http://user:" + secret + "@collector\n:4317/api/v2/spans"}

	var err error

	out := testutil.StdoutOutputForFunc(func() {
		_, err = buildZipkinExporter(t.Context(), cfg, logging.NewMockLogger(logging.DEBUG))
	})

	require.Error(t, err)
	require.NotContains(t, err.Error(), secret)
	require.Contains(t, err.Error(), "REDACTED")
	require.NotContains(t, out, secret)
}
